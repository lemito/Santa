using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Logging.Abstractions;
using santa.Application;
using santa.Data;
using santa.Infrastructure;

static void Check(bool condition, string message) { if (!condition) throw new Exception(message); }
static async Task Error(Func<Task> action, string code)
{
    try { await action(); throw new Exception("Expected " + code); }
    catch (GameException e) { Check(e.Code == code,$"Expected {code}, got {e.Code}"); }
}
var native = new NativeAssignmentEngine();
var ids = Enumerable.Range(0,5).Select(_ => Guid.NewGuid()).ToArray();
var result = native.Draw(new(ids,[new(ids[0],ids[1])]));
Check(result.Status == EngineStatus.Success,"Native P/Invoke");
Check(result.Assignments!.Count == 5 && result.Assignments.Values.Distinct().Count() == 5,"UUID round trip bijection");
Check(result.Assignments[ids[0]] != ids[1] && result.Assignments[ids[1]] != ids[0],"Symmetric exclusions");
Check(native.Draw(new([ids[0],ids[0]],[])).Status == EngineStatus.InvalidInput,"Duplicate UUID");
Check(native.Draw(new(ids,[new(ids[0],Guid.NewGuid())])).Status == EngineStatus.InvalidInput,"Unknown UUID");
Check(native.Draw(new([ids[0],ids[1]],[new(ids[0],ids[1])])).Status == EngineStatus.Unsatisfiable,"Unsatisfiable");
var connection = Environment.GetEnvironmentVariable("SANTA_TEST_DB")
    ?? throw new Exception("SANTA_TEST_DB must point to a disposable PostgreSQL database");
var options = new DbContextOptionsBuilder<SantaDb>().UseNpgsql(connection).Options;
var factory = new Factory(options);
await using (var db = new SantaDb(options)) {
    await db.Database.MigrateAsync();
    // Only this test database is cleared; the environment variable is mandatory.
    await db.Database.ExecuteSqlRawAsync("TRUNCATE TABLE \"Assignments\", \"Draws\", \"Exclusions\", \"Participants\", \"Games\", \"AspNetUsers\" CASCADE");
    db.Users.AddRange(ids.Select((id,i) => new Account { Id = id,UserName = $"person{i}@example.test",NormalizedUserName = $"PERSON{i}@EXAMPLE.TEST",Email = $"person{i}@example.test",NormalizedEmail = $"PERSON{i}@EXAMPLE.TEST",DisplayName = $"Person {i}" }));
    await db.SaveChangesAsync();
}
var service = new GameService(new EfGameStore(factory),native);
var emails = Enumerable.Range(0,5).Select(i => $"person{i}@example.test").ToArray();
var owner = ids[0];
async Task<(Guid,PublishInput)> Game()
{
    var id = await service.Create(owner,"Test gift game");
    await service.Configure(owner,id,new(1,emails,[]));
    return (id,new(2,Guid.NewGuid()));
}
var (game,request) = await Game();
await Error(() => service.Configure(ids[1],game,new(2,emails,[])),"forbidden");
await Error(() => service.Configure(owner,game,new(1,emails,[])),"configuration_conflict");
await Error(() => service.Configure(owner,game,new(2,[emails[0],emails[0]],[])),"duplicate_participants");
await Error(() => service.Configure(owner,game,new(2,["missing@example.test"],[])),"unknown_account");
await Error(() => service.MyAssignment(owner,game),"assignment_unavailable");
const string markdown = "## Идеи\n\n- **Книга**\n- [Сайт](https://example.test)\n\n![Подарок](https://example.test/gift.png)\n\n![Опасно](data:text/html;base64,PHNjcmlwdD4=)\n\n<script>alert(1)</script>\n\n[опасная ссылка](javascript:alert(1))";
await service.SetMessage(ids[1],game,markdown);
var ownMessage = await service.MyMessage(ids[1],game);
Check(ownMessage.Markdown == markdown,"Message markdown round trip");
Check(ownMessage.Html.Contains("<h2") && ownMessage.Html.Contains("<strong>Книга</strong>"),"Markdown formatting");
Check(ownMessage.Html.Contains("<img") && ownMessage.Html.Contains("https://example.test/gift.png"),"Markdown HTTPS image");
Check(!ownMessage.Html.Contains("<script",StringComparison.OrdinalIgnoreCase) &&
    !ownMessage.Html.Contains("javascript:",StringComparison.OrdinalIgnoreCase) &&
    !ownMessage.Html.Contains("data:",StringComparison.OrdinalIgnoreCase),"Markdown XSS sanitizing");
await Error(() => service.SetMessage(Guid.NewGuid(),game,"not allowed"),"not_participant");
await Error(() => service.MyMessage(Guid.NewGuid(),game),"not_participant");
await Error(() => service.SetMessage(ids[1],game,new string('x',5001)),"invalid_message");
var s1 = (await service.Capture(owner,game,request))!;
var request2 = new PublishInput(2,Guid.NewGuid());
var s2 = (await service.Capture(owner,game,request2))!;
var draw1 = native.Draw(s1.Input).Assignments!;
var draw2 = native.Draw(s2.Input).Assignments!;
async Task<bool> TryCommit(PublishInput r, Snapshot s, IReadOnlyDictionary<Guid,Guid> a) {
    try { await service.Commit(owner,r,s,a); return true; }
    catch (GameException e) when (e.Code == "already_published") { return false; }
}
var race = await Task.WhenAll(TryCommit(request,s1,draw1),TryCommit(request2,s2,draw2));
Check(race.Count(x => x) == 1,"Exactly one concurrent publication");
var winning = race[0] ? request : request2;
await service.Publish(owner,game,winning);
await Error(() => service.Publish(owner,game,new(999,winning.RequestKey)),"already_published");
await Error(() => service.Configure(owner,game,new(2,emails,[])),"configuration_conflict");
await Error(() => service.SetRecipientMessage(ids[1],game,new string('x',5001)),"invalid_message");
await using (var db = new SantaDb(options)) {
    Check(await db.Draws.CountAsync(d => d.GameId == game) == 1,"One draw");
    Check(await db.Assignments.CountAsync(d => d.GameId == game) == 5,"Complete result");
}
for (int i = 0; i < ids.Length; ++i) {
    var recipient = await service.MyAssignment(ids[i],game);
    Check(recipient.Email != emails[i],"Only own recipient");
}
await using (var db = new SantaDb(options)) {
    var giver = await db.Assignments.Where(a => a.GameId == game && a.ReceiverId == ids[1]).Select(a => a.GiverId).SingleAsync();
    var recipient = await service.MyAssignment(giver,game);
    Check(recipient.MessageHtml.Contains("<strong>Книга</strong>"),"Giver sees receiver message");
    const string reply = "## От Санты\n\n**Сюрприз готов**\n\n![Ёлка](https://example.test/tree.png)\n\n![Опасно](javascript:alert(1))";
    await service.SetRecipientMessage(giver,game,reply);
    var outgoing = await service.MyRecipientMessage(giver,game);
    Check(outgoing.Markdown == reply && outgoing.Html.Contains("<strong>Сюрприз готов</strong>"),"Recipient message round trip");
    var incoming = await service.MessageFromSanta(ids[1],game);
    Check(incoming.Html.Contains("https://example.test/tree.png") &&
        !incoming.Html.Contains("javascript:",StringComparison.OrdinalIgnoreCase),"Recipient sees safe anonymous Santa message");
}
await Error(() => service.SetRecipientMessage(Guid.NewGuid(),game,"not allowed"),"not_participant");
await Error(() => service.MyRecipientMessage(Guid.NewGuid(),game),"assignment_unavailable");
await Error(() => service.MessageFromSanta(Guid.NewGuid(),game),"assignment_unavailable");
await Error(() => service.MyAssignment(Guid.NewGuid(),game),"assignment_unavailable");
await service.Close(owner,game);
await Error(() => service.MyAssignment(owner,game),"assignment_unavailable");
await Error(() => service.SetMessage(ids[1],game,"too late"),"message_closed");
await Error(() => service.SetRecipientMessage(ids[1],game,"too late"),"message_closed");
await Error(() => service.MessageFromSanta(ids[1],game),"assignment_unavailable");
// Snapshot becomes stale during the calculation.
var (stale,staleRequest) = await Game();
var snapshot = (await service.Capture(owner,stale,staleRequest))!;
await service.Configure(owner,stale,new(2,emails,[]));
await Error(() => service.Commit(owner,staleRequest,snapshot,native.Draw(snapshot.Input).Assignments!),"configuration_conflict");
// Same request racing is idempotent as well.
var (same,sameRequest) = await Game();
var sameSnapshot = (await service.Capture(owner,same,sameRequest))!;
await Task.WhenAll(service.Commit(owner,sameRequest,sameSnapshot,native.Draw(sameSnapshot.Input).Assignments!),
    service.Commit(owner,sameRequest,sameSnapshot,native.Draw(sameSnapshot.Input).Assignments!));
// Invalid engine output never reaches storage.
var (invalid,invalidRequest) = await Game();
var invalidSnapshot = (await service.Capture(owner,invalid,invalidRequest))!;
await Error(() => service.Commit(owner,invalidRequest,invalidSnapshot,ids.ToDictionary(x => x,x => x)),"invalid_engine_result");
// Inject a real PostgreSQL failure during assignment inserts: game/draw roll back.
var (rollback,rollbackRequest) = await Game();
await using (var db = new SantaDb(options)) {
    await db.Database.ExecuteSqlRawAsync("CREATE OR REPLACE FUNCTION santa_test_fail() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''injected test failure''; END'; CREATE TRIGGER santa_test_fail BEFORE INSERT ON \"Assignments\" FOR EACH ROW EXECUTE FUNCTION santa_test_fail()");
}
try {
    try { await service.Publish(owner,rollback,rollbackRequest); throw new Exception("Expected database failure"); }
    catch (DbUpdateException) { }
} finally {
    await using var db = new SantaDb(options);
    await db.Database.ExecuteSqlRawAsync("DROP TRIGGER santa_test_fail ON \"Assignments\"; DROP FUNCTION santa_test_fail()");
}
await using (var db = new SantaDb(options)) {
    foreach (var id in new[] { stale,invalid,rollback }) {
        Check((await db.Games.FindAsync(id))!.State == "Draft","Game remains draft");
        Check(!await db.Draws.AnyAsync(d => d.GameId == id),"No partial draw");
        Check(!await db.Assignments.AnyAsync(a => a.GameId == id),"No partial assignments");
    }
}
await service.Publish(owner,rollback,rollbackRequest);
Console.WriteLine("PASS: P/Invoke, assignments, messages, safe Markdown, PostgreSQL migration, concurrency, rollback, rights and lifecycle");
sealed class Factory(DbContextOptions<SantaDb> options) : IDbContextFactory<SantaDb>
{
    public SantaDb CreateDbContext() => new(options);
}
