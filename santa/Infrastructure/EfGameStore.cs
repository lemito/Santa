using System.Data;
using Microsoft.EntityFrameworkCore;
using santa.Data;

using santa.Application;

namespace santa.Infrastructure;

public sealed class EfGameStore(IDbContextFactory<SantaDb> factory) : IGameStore
{
    private static void Owner(Game game, Guid actor)
    { if (game.OwnerId != actor) throw new GameException("forbidden", 403); }
    private static void Editable(Game game, long version)
    { if (game.State != "Draft" || game.Version != version) throw new GameException("configuration_conflict", 409); }
    private static async Task<Game> Lock(SantaDb db, Guid id)
        => await db.Games.FromSqlInterpolated($"SELECT * FROM \"Games\" WHERE \"Id\" = {id} FOR UPDATE")
            .SingleOrDefaultAsync() ?? throw new GameException("not_found", 404);
    public async Task<Guid> Create(Guid actor, string title)
    {
        title = title.Trim();
        if (title.Length is < 1 or > 150) throw new GameException("invalid_title");
        await using var db = await factory.CreateDbContextAsync();
        var game = new Game { Id = Guid.NewGuid(), OwnerId = actor, Title = title };
        db.Add(game); await db.SaveChangesAsync(); return game.Id;
    }
    public async Task<GameView[]> List(Guid actor)
    {
        await using var db = await factory.CreateDbContextAsync();
        return await db.Games.Where(g => g.OwnerId == actor || db.Participants.Any(p => p.GameId == g.Id && p.AccountId == actor))
            .OrderBy(g => g.Title).Select(g => new GameView(g.Id,g.Title,g.State,g.Version,g.OwnerId == actor)).ToArrayAsync();
    }
    public async Task<GameView> Get(Guid actor, Guid id)
    {
        await using var db = await factory.CreateDbContextAsync();
        var game = await db.Games.FindAsync(id) ?? throw new GameException("not_found",404);
        if (game.OwnerId != actor && !await db.Participants.AnyAsync(p => p.GameId == id && p.AccountId == actor))
            throw new GameException("not_found",404);
        return new(game.Id,game.Title,game.State,game.Version,game.OwnerId == actor);
    }
    public async Task<ConfigurationView> Configuration(Guid actor, Guid id)
    {
        await using var db = await factory.CreateDbContextAsync();
        await using var tx = await db.Database.BeginTransactionAsync(IsolationLevel.RepeatableRead);
        var game = await db.Games.FindAsync(id) ?? throw new GameException("not_found",404);
        Owner(game,actor);
        var participants = await db.Participants.Where(p => p.GameId == id).OrderBy(p => p.AccountId)
            .Select(p => new ParticipantView(p.AccountId,p.Account.Email!,p.Account.DisplayName)).ToArrayAsync();
        var emails = participants.ToDictionary(p => p.Id,p => p.Email);
        var pairs = await db.Exclusions.Where(p => p.GameId == id).ToArrayAsync();
        var view = new ConfigurationView(participants,pairs.Select(p => new PairEmails(emails[p.FirstId],emails[p.SecondId])).ToArray());
        await tx.CommitAsync();
        return view;
    }
    public async Task Configure(Guid actor, Guid id, ConfigurationInput input)
    {
        if (input.Emails is null || input.Exclusions is null ||
            input.Emails.Any(e => string.IsNullOrWhiteSpace(e) || e.Trim().Length > 256))
            throw new GameException("invalid_configuration");
        var normalized = input.Emails.Select(e => e.Trim().ToUpperInvariant()).ToArray();
        if (normalized.Distinct().Count() != normalized.Length) throw new GameException("duplicate_participants");
        await using var db = await factory.CreateDbContextAsync();
        await using var tx = await db.Database.BeginTransactionAsync();
        var game = await Lock(db,id); Owner(game,actor); Editable(game,input.Version);
        var accounts = await db.Users.Where(u => normalized.Contains(u.NormalizedEmail!)).ToArrayAsync();
        if (accounts.Length != normalized.Length) throw new GameException("unknown_account");
        var map = accounts.ToDictionary(u => u.NormalizedEmail!,u => u.Id);
        var pairs = new HashSet<(Guid First,Guid Second)>();
        foreach (var p in input.Exclusions)
        {
            if (p is null || !map.TryGetValue(p.First?.Trim().ToUpperInvariant() ?? "", out var a) ||
                !map.TryGetValue(p.Second?.Trim().ToUpperInvariant() ?? "",out var b) || a == b)
                throw new GameException("invalid_exclusion");
            pairs.Add(a.CompareTo(b) < 0 ? (a,b) : (b,a));
        }
        await db.Exclusions.Where(p => p.GameId == id).ExecuteDeleteAsync();
        await db.Participants.Where(p => p.GameId == id).ExecuteDeleteAsync();
        db.Participants.AddRange(accounts.Select(a => new Participant { GameId = id,AccountId = a.Id }));
        db.Exclusions.AddRange(pairs.Select(p => new ExcludedPair { GameId = id,FirstId = p.First,SecondId = p.Second }));
        game.Version = checked(game.Version + 1);
        await db.SaveChangesAsync(); await tx.CommitAsync();
    }
    public async Task<Snapshot?> Capture(Guid actor, Guid id, PublishInput request)
    {
        if (request.RequestKey == Guid.Empty) throw new GameException("invalid_request_key");
        await using var db = await factory.CreateDbContextAsync();
        // A short lock provides a coherent snapshot. Released before native work.
        await using var tx = await db.Database.BeginTransactionAsync();
        var game = await Lock(db,id); Owner(game,actor);
        var prior = await db.Draws.FindAsync(id);
        if (prior is not null)
        {
            if (prior.RequestKey == request.RequestKey && prior.Version == request.Version) return null;
            throw new GameException("already_published",409);
        }
        Editable(game,request.Version);
        var ids = await db.Participants.Where(p => p.GameId == id).OrderBy(p => p.AccountId).Select(p => p.AccountId).ToArrayAsync();
        var pairs = await db.Exclusions.Where(p => p.GameId == id).Select(p => new Exclusion(p.FirstId,p.SecondId)).ToArrayAsync();
        await tx.CommitAsync();
        return new(id,game.Version,new(ids,pairs));
    }
    public async Task Commit(Guid actor, PublishInput request, Snapshot snapshot, IReadOnlyDictionary<Guid,Guid> assignments)
    {
        await using var db = await factory.CreateDbContextAsync();
        await using var tx = await db.Database.BeginTransactionAsync();
        var game = await Lock(db,snapshot.GameId); Owner(game,actor);
        var prior = await db.Draws.FindAsync(game.Id);
        if (prior is not null)
        {
            if (prior.RequestKey == request.RequestKey && prior.Version == request.Version) return;
            throw new GameException("already_published",409);
        }
        Editable(game,snapshot.Version);
        if (request.Version != snapshot.Version) throw new GameException("configuration_conflict",409);
        db.Draws.Add(new Draw { GameId = game.Id,Version = snapshot.Version,RequestKey = request.RequestKey });
        db.Assignments.AddRange(assignments.Select(p => new Assignment { GameId = game.Id,GiverId = p.Key,ReceiverId = p.Value }));
        game.State = "Published";
        await db.SaveChangesAsync(); await tx.CommitAsync();
    }
    public async Task Close(Guid actor, Guid id)
    {
        await using var db = await factory.CreateDbContextAsync();
        await using var tx = await db.Database.BeginTransactionAsync();
        var game = await Lock(db,id); Owner(game,actor);
        if (game.State != "Published") throw new GameException("state_conflict",409);
        game.State = "Closed"; await db.SaveChangesAsync(); await tx.CommitAsync();
    }
    public async Task<RecipientView> MyAssignment(Guid actor, Guid id)
    {
        await using var db = await factory.CreateDbContextAsync();
        var recipient = await (from a in db.Assignments
            join g in db.Games on a.GameId equals g.Id
            join u in db.Users on a.ReceiverId equals u.Id
            join p in db.Participants on new { a.GameId, AccountId = a.ReceiverId } equals new { p.GameId, p.AccountId }
            where a.GameId == id && a.GiverId == actor && g.State == "Published"
            select new { u.DisplayName, u.Email, p.SantaMessage }).SingleOrDefaultAsync();
        if (recipient is null) throw new GameException("assignment_unavailable",404);
        return new(recipient.DisplayName,recipient.Email!,SafeMarkdown.Render(recipient.SantaMessage));
    }
    public async Task<ParticipantMessageView> MyMessage(Guid actor, Guid id)
    {
        await using var db = await factory.CreateDbContextAsync();
        var message = await db.Participants
            .Where(p => p.GameId == id && p.AccountId == actor)
            .Select(p => p.SantaMessage).SingleOrDefaultAsync();
        if (message is null) throw new GameException("not_participant",404);
        return new(message,SafeMarkdown.Render(message));
    }
    public async Task SetMessage(Guid actor, Guid id, string message)
    {
        message = (message ?? "").Trim();
        if (message.Length > 5000) throw new GameException("invalid_message");
        await using var db = await factory.CreateDbContextAsync();
        await using var tx = await db.Database.BeginTransactionAsync();
        var game = await Lock(db,id);
        if (game.State == "Closed") throw new GameException("message_closed",409);
        var participant = await db.Participants.FindAsync(id,actor)
            ?? throw new GameException("not_participant",403);
        participant.SantaMessage = message;
        await db.SaveChangesAsync(); await tx.CommitAsync();
    }
    public async Task<ParticipantMessageView> MyRecipientMessage(Guid actor, Guid id)
    {
        await using var db = await factory.CreateDbContextAsync();
        var message = await (from a in db.Assignments
            join g in db.Games on a.GameId equals g.Id
            where a.GameId == id && a.GiverId == actor && g.State == "Published"
            select a.RecipientMessage).SingleOrDefaultAsync();
        if (message is null) throw new GameException("assignment_unavailable",404);
        return new(message,SafeMarkdown.Render(message));
    }
    public async Task SetRecipientMessage(Guid actor, Guid id, string message)
    {
        message = (message ?? "").Trim();
        if (message.Length > 5000) throw new GameException("invalid_message");
        await using var db = await factory.CreateDbContextAsync();
        await using var tx = await db.Database.BeginTransactionAsync();
        var game = await Lock(db,id);
        if (game.State != "Published") throw new GameException(
            game.State == "Closed" ? "message_closed" : "message_not_published",409);
        var assignment = await db.Assignments.FindAsync(id,actor)
            ?? throw new GameException("not_participant",403);
        assignment.RecipientMessage = message;
        await db.SaveChangesAsync(); await tx.CommitAsync();
    }
    public async Task<ParticipantMessageView> MessageFromSanta(Guid actor, Guid id)
    {
        await using var db = await factory.CreateDbContextAsync();
        var message = await (from a in db.Assignments
            join g in db.Games on a.GameId equals g.Id
            where a.GameId == id && a.ReceiverId == actor && g.State == "Published"
            select a.RecipientMessage).SingleOrDefaultAsync();
        if (message is null) throw new GameException("assignment_unavailable",404);
        return new(message,SafeMarkdown.Render(message));
    }
}
