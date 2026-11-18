using System.Security.Claims;
using Microsoft.AspNetCore.Antiforgery;
using Microsoft.AspNetCore.Identity;
using Microsoft.AspNetCore.Mvc;
using santa.Application;
using santa.Data;

namespace santa.Infrastructure;
public static class WebEndpoints
{
    public static Guid Actor(ClaimsPrincipal principal) => Guid.Parse(principal.FindFirstValue(ClaimTypes.NameIdentifier)!);
    private static string[] Lines(string? text) => (text ?? "").Split(['\r','\n'],StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
    public static void MapSanta(this WebApplication app)
    {
        app.MapGet("/health", () => Results.Ok(new { status = "ok" }));
        app.MapGet("/health/ready", async (Microsoft.EntityFrameworkCore.IDbContextFactory<SantaDb> factory) => {
            await using var db = await factory.CreateDbContextAsync();
            return await db.Database.CanConnectAsync() ? Results.Ok(new { status = "ready" }) : Results.StatusCode(503);
        });
        app.MapGet("/api/session", (HttpContext ctx, IAntiforgery antiforgery) => {
            ctx.Response.Headers.CacheControl = "no-store";
            var token = antiforgery.GetAndStoreTokens(ctx);
            return Results.Ok(new { authenticated = ctx.User.Identity?.IsAuthenticated == true, csrf = token.RequestToken });
        });
        // Form binding enables ASP.NET's antiforgery validation before handlers.
        app.MapPost("/account/register", async ([FromForm] string email, [FromForm] string password,
            [FromForm] string name, UserManager<Account> users, SignInManager<Account> signIn) => {
            if (string.IsNullOrWhiteSpace(name) || name.Trim().Length > 100 || email.Length > 256 || password.Length > 256)
                return Results.LocalRedirect("/register?error=invalid_account");
            var user = new Account { Id = Guid.NewGuid(),UserName = email.Trim(),Email = email.Trim(),DisplayName = name.Trim() };
            var result = await users.CreateAsync(user,password);
            if (!result.Succeeded) return Results.LocalRedirect("/register?error=registration_failed");
            await signIn.SignInAsync(user,isPersistent:false);
            return Results.LocalRedirect("/");
        }).RequireRateLimiting("auth");
        app.MapPost("/account/login", async ([FromForm] string email, [FromForm] string password,
            SignInManager<Account> signIn) => {
            if (email.Length > 256 || password.Length > 256) return Results.LocalRedirect("/login?error=login_failed");
            var result = await signIn.PasswordSignInAsync(email.Trim(),password,false,lockoutOnFailure:true);
            return Results.LocalRedirect(result.Succeeded ? "/" : "/login?error=login_failed");
        }).RequireRateLimiting("auth");
        app.MapPost("/account/logout", async (HttpContext context, IAntiforgery csrf, SignInManager<Account> signIn) => {
            await csrf.ValidateRequestAsync(context);
            await signIn.SignOutAsync(); return Results.LocalRedirect("/");
        }).RequireAuthorization();
        var api = app.MapGroup("/api/games").RequireAuthorization();
        api.MapGet("/", (ClaimsPrincipal u, GameService games) => games.List(Actor(u)));
        api.MapPost("/", async (CreateGame request, ClaimsPrincipal u, GameService games) =>
            Results.Ok(new { id = await games.Create(Actor(u),request.Title ?? "") }));
        api.MapGet("/{id:guid}", (Guid id, ClaimsPrincipal u, GameService games) => games.Get(Actor(u),id));
        api.MapGet("/{id:guid}/configuration", (Guid id, ClaimsPrincipal u, GameService games) => games.Configuration(Actor(u),id));
        api.MapPut("/{id:guid}/configuration", async (Guid id, ConfigurationInput request, ClaimsPrincipal u, GameService games) => {
            await games.Configure(Actor(u),id,request); return Results.NoContent();
        });
        api.MapPost("/{id:guid}/publish", async (Guid id, PublishInput request, ClaimsPrincipal u, GameService games) => {
            await games.Publish(Actor(u),id,request); return Results.Ok(new { state = (await games.Get(Actor(u),id)).State });
        }).RequireRateLimiting("draw");
        api.MapPost("/{id:guid}/close", async (Guid id, ClaimsPrincipal u, GameService games) => {
            await games.Close(Actor(u),id); return Results.NoContent();
        });
        api.MapGet("/{id:guid}/my-assignment", (Guid id, ClaimsPrincipal u, GameService games) => games.MyAssignment(Actor(u),id));
        api.MapGet("/{id:guid}/my-message", (Guid id, ClaimsPrincipal u, GameService games) => games.MyMessage(Actor(u),id));
        api.MapPut("/{id:guid}/my-message", async (Guid id, MessageInput request, ClaimsPrincipal u, GameService games) => {
            await games.SetMessage(Actor(u),id,request.Message ?? ""); return Results.NoContent();
        });
        api.MapGet("/{id:guid}/my-recipient-message", (Guid id, ClaimsPrincipal u, GameService games) => games.MyRecipientMessage(Actor(u),id));
        api.MapPut("/{id:guid}/my-recipient-message", async (Guid id, MessageInput request, ClaimsPrincipal u, GameService games) => {
            await games.SetRecipientMessage(Actor(u),id,request.Message ?? ""); return Results.NoContent();
        });
        api.MapGet("/{id:guid}/message-from-santa", (Guid id, ClaimsPrincipal u, GameService games) => games.MessageFromSanta(Actor(u),id));
        var forms = app.MapGroup("/actions").RequireAuthorization();
        forms.MapPost("/create", async ([FromForm] string title, ClaimsPrincipal u, GameService games) =>
            Results.LocalRedirect($"/games/{await games.Create(Actor(u),title)}"));
        forms.MapPost("/{id:guid}/configure", async (Guid id, [FromForm] long version, [FromForm] string? emails,
            [FromForm] string? exclusions, ClaimsPrincipal u, GameService games) => {
            var pairs = Lines(exclusions).Select(line => line.Split(',', StringSplitOptions.TrimEntries)).ToArray();
            if (pairs.Any(p => p.Length != 2)) throw new GameException("invalid_exclusion");
            await games.Configure(Actor(u),id,new(version,Lines(emails),pairs.Select(p => new PairEmails(p[0],p[1])).ToArray()));
            return Results.LocalRedirect($"/games/{id}");
        });
        forms.MapPost("/{id:guid}/publish", async (Guid id, [FromForm] long version, [FromForm] Guid requestKey,
            ClaimsPrincipal u, GameService games) => {
            await games.Publish(Actor(u),id,new(version,requestKey)); return Results.LocalRedirect($"/games/{id}");
        }).RequireRateLimiting("draw");
        forms.MapPost("/{id:guid}/close", async (Guid id, HttpContext ctx, IAntiforgery csrf, GameService games) => {
            await csrf.ValidateRequestAsync(ctx);
            await games.Close(Actor(ctx.User),id); return Results.LocalRedirect($"/games/{id}");
        });
        forms.MapPost("/{id:guid}/message", async (Guid id, [FromForm] string? message,
            ClaimsPrincipal u, GameService games) => {
            await games.SetMessage(Actor(u),id,message ?? ""); return Results.LocalRedirect($"/games/{id}");
        });
        forms.MapPost("/{id:guid}/recipient-message", async (Guid id, [FromForm] string? message,
            ClaimsPrincipal u, GameService games) => {
            await games.SetRecipientMessage(Actor(u),id,message ?? ""); return Results.LocalRedirect($"/games/{id}");
        });
    }
    public record CreateGame(string Title);
    public record MessageInput(string? Message);
}
