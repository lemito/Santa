using Microsoft.AspNetCore.Identity;
namespace santa.Data;
public sealed class Account : IdentityUser<Guid> { public string DisplayName { get; set; } = ""; }
public sealed class Game
{
    public Guid Id { get; set; }
    public Guid OwnerId { get; set; }
    public string Title { get; set; } = "";
    public string State { get; set; } = "Draft";
    public long Version { get; set; } = 1;
    public long BudgetMinor { get; set; }
    public string Currency { get; set; } = "EUR";
    public string BudgetNote { get; set; } = "";
}
public sealed class Participant
{
    public Guid GameId { get; set; }
    public Guid AccountId { get; set; }
    public string SantaMessage { get; set; } = "";
    public Account Account { get; set; } = null!;
}
public sealed class ExcludedPair
{
    public Guid GameId { get; set; }
    public Guid FirstId { get; set; }
    public Guid SecondId { get; set; }
}
public sealed class Draw
{
    public Guid GameId { get; set; }
    public long Version { get; set; }
    public Guid RequestKey { get; set; }
    public string EngineVersion { get; set; } = "uniform-hybrid-v1";
    public string InputDigest { get; set; } = "";
    public string ResultDigest { get; set; } = "";
    public int ParticipantCount { get; set; }
    public int ExclusionCount { get; set; }
    public DateTimeOffset PublishedAt { get; set; }
}
public sealed class Assignment
{
    public Guid GameId { get; set; }
    public Guid GiverId { get; set; }
    public Guid ReceiverId { get; set; }
    public string RecipientMessage { get; set; } = "";
}
