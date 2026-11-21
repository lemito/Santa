namespace santa.Application;

public enum EngineStatus { Success, InvalidInput, Unsatisfiable, SearchLimitExceeded, Cancelled, InternalError, ResourceLimitExceeded }
public record Exclusion(Guid First, Guid Second);
public record EngineInput(Guid[] Participants, Exclusion[] Exclusions);
public record EngineResult(EngineStatus Status, IReadOnlyDictionary<Guid, Guid>? Assignments = null);
public interface IAssignmentEngine { EngineResult Draw(EngineInput input); }
public sealed class GameException(string code, int status = 400) : Exception(code)
{
    public string Code => Message;
    public int Status => status;
}
public record PairEmails(string First, string Second);
public record ConfigurationInput(long Version, string[] Emails, PairEmails[] Exclusions, long BudgetMinor = 0, string Currency = "EUR", string BudgetNote = "");
public record PublishInput(long Version, Guid RequestKey);
public record ParticipantView(Guid Id, string Email, string Name);
public record GameView(Guid Id, string Title, string State, long Version, bool IsOwner, long BudgetMinor = 0, string Currency = "EUR", string BudgetNote = "", DrawReceipt? Receipt = null);
public record DrawReceipt(string EngineVersion, string InputDigest, string ResultDigest, int ParticipantCount, int ExclusionCount, DateTimeOffset PublishedAt);
public record ConfigurationView(ParticipantView[] Participants, PairEmails[] Exclusions);
public record ParticipantMessageView(string Markdown, string Html);
public record RecipientView(string Name, string Email, string MessageHtml);
public record Snapshot(Guid GameId, long Version, EngineInput Input);
