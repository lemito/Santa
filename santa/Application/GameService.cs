namespace santa.Application;

public interface IGameStore
{
    Task<Guid> Create(Guid actor, string title);
    Task<GameView[]> List(Guid actor);
    Task<GameView> Get(Guid actor, Guid id);
    Task<ConfigurationView> Configuration(Guid actor, Guid id);
    Task Configure(Guid actor, Guid id, ConfigurationInput input);
    Task<Snapshot?> Capture(Guid actor, Guid id, PublishInput request);
    Task Commit(Guid actor, PublishInput request, Snapshot snapshot, IReadOnlyDictionary<Guid, Guid> assignments);
    Task Close(Guid actor, Guid id);
    Task<RecipientView> MyAssignment(Guid actor, Guid id);
    Task<ParticipantMessageView> MyMessage(Guid actor, Guid id);
    Task SetMessage(Guid actor, Guid id, string message);
    Task<ParticipantMessageView> MyRecipientMessage(Guid actor, Guid id);
    Task SetRecipientMessage(Guid actor, Guid id, string message);
    Task<ParticipantMessageView> MessageFromSanta(Guid actor, Guid id);
}

public sealed class GameService(IGameStore store, IAssignmentEngine engine)
{
    public Task<Guid> Create(Guid actor, string title) => store.Create(actor,title);
    public Task<GameView[]> List(Guid actor) => store.List(actor);
    public Task<GameView> Get(Guid actor, Guid id) => store.Get(actor,id);
    public Task<ConfigurationView> Configuration(Guid actor, Guid id) => store.Configuration(actor,id);
    public Task Configure(Guid actor, Guid id, ConfigurationInput input) => store.Configure(actor,id,input);
    public Task<Snapshot?> Capture(Guid actor, Guid id, PublishInput request) => store.Capture(actor,id,request);
    public Task Close(Guid actor, Guid id) => store.Close(actor,id);
    public Task<RecipientView> MyAssignment(Guid actor, Guid id) => store.MyAssignment(actor,id);
    public Task<ParticipantMessageView> MyMessage(Guid actor, Guid id) => store.MyMessage(actor,id);
    public Task SetMessage(Guid actor, Guid id, string message) => store.SetMessage(actor,id,message);
    public Task<ParticipantMessageView> MyRecipientMessage(Guid actor, Guid id) => store.MyRecipientMessage(actor,id);
    public Task SetRecipientMessage(Guid actor, Guid id, string message) => store.SetRecipientMessage(actor,id,message);
    public Task<ParticipantMessageView> MessageFromSanta(Guid actor, Guid id) => store.MessageFromSanta(actor,id);
    public async Task Publish(Guid actor, Guid id, PublishInput request)
    {
        var snapshot = await Capture(actor,id,request);
        if (snapshot is null) return;
        var result = engine.Draw(snapshot.Input);
        if (result.Status != EngineStatus.Success)
            throw new GameException(result.Status switch {
                EngineStatus.Unsatisfiable => "unsatisfiable",
                EngineStatus.SearchLimitExceeded => "search_limit",
                EngineStatus.ResourceLimitExceeded => "resource_limit",
                EngineStatus.InvalidInput => "invalid_configuration", _ => "engine_error"
            }, result.Status == EngineStatus.InternalError ? 503 : 422);
        await Commit(actor,request,snapshot,result.Assignments!);
    }
    public async Task Commit(Guid actor, PublishInput request, Snapshot snapshot, IReadOnlyDictionary<Guid, Guid> assignments)
    {
        // Validate again here: every implementation of the port must respect this.
        var ids = snapshot.Input.Participants.ToHashSet();
        if (assignments.Count != ids.Count || !assignments.Keys.ToHashSet().SetEquals(ids) ||
            !assignments.Values.ToHashSet().SetEquals(ids) || assignments.Any(p => p.Key == p.Value) ||
            snapshot.Input.Exclusions.Any(p => assignments[p.First] == p.Second || assignments[p.Second] == p.First))
            throw new GameException("invalid_engine_result",503);
        await store.Commit(actor,request,snapshot,assignments);
    }
}
