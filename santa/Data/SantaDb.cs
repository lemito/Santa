using Microsoft.AspNetCore.Identity;
using Microsoft.AspNetCore.Identity.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore;
namespace santa.Data;
public sealed class SantaDb(DbContextOptions<SantaDb> options) : IdentityDbContext<Account, IdentityRole<Guid>, Guid>(options)
{
    public DbSet<Game> Games => Set<Game>();
    public DbSet<Participant> Participants => Set<Participant>();
    public DbSet<ExcludedPair> Exclusions => Set<ExcludedPair>();
    public DbSet<Draw> Draws => Set<Draw>();
    public DbSet<Assignment> Assignments => Set<Assignment>();
    protected override void OnModelCreating(ModelBuilder b)
    {
        base.OnModelCreating(b);
        b.Entity<Account>().Property(x => x.DisplayName).HasMaxLength(100);
        b.Entity<Game>(e => {
            e.HasKey(x => x.Id); e.Property(x => x.Title).HasMaxLength(150);
            e.Property(x => x.State).HasMaxLength(16);
            e.Property(x => x.Currency).HasMaxLength(3).IsRequired();
            e.Property(x => x.BudgetNote).HasMaxLength(500);
            e.ToTable(t => t.HasCheckConstraint("nonnegative_budget", "\"BudgetMinor\" >= 0"));
            e.HasOne<Account>().WithMany().HasForeignKey(x => x.OwnerId).OnDelete(DeleteBehavior.Restrict);
            e.ToTable(t => t.HasCheckConstraint("game_state", "\"State\" IN ('Draft','Published','Closed')"));
        });
        b.Entity<Participant>(e => {
            e.HasKey(x => new { x.GameId, x.AccountId });
            e.Property(x => x.SantaMessage).HasMaxLength(5000);
            e.HasOne<Game>().WithMany().HasForeignKey(x => x.GameId).OnDelete(DeleteBehavior.Cascade);
            e.HasOne(x => x.Account).WithMany().HasForeignKey(x => x.AccountId).OnDelete(DeleteBehavior.Restrict);
        });
        b.Entity<ExcludedPair>(e => {
            e.HasKey(x => new { x.GameId, x.FirstId, x.SecondId });
            e.HasOne<Participant>().WithMany().HasForeignKey(x => new { x.GameId, x.FirstId }).OnDelete(DeleteBehavior.Cascade);
            e.HasOne<Participant>().WithMany().HasForeignKey(x => new { x.GameId, x.SecondId }).OnDelete(DeleteBehavior.Cascade);
            e.ToTable(t => t.HasCheckConstraint("different_exclusion", "\"FirstId\" <> \"SecondId\""));
        });
        b.Entity<Draw>(e => {
            e.HasKey(x => x.GameId);
            e.Property(x => x.InputDigest).HasMaxLength(64);
            e.Property(x => x.ResultDigest).HasMaxLength(64);
            e.HasOne<Game>().WithOne().HasForeignKey<Draw>(x => x.GameId).OnDelete(DeleteBehavior.Restrict);
        });
        b.Entity<Assignment>(e => {
            e.HasKey(x => new { x.GameId, x.GiverId });
            e.Property(x => x.RecipientMessage).HasMaxLength(5000);
            e.HasIndex(x => new { x.GameId, x.ReceiverId }).IsUnique();
            e.HasOne<Draw>().WithMany().HasForeignKey(x => x.GameId).OnDelete(DeleteBehavior.Cascade);
            e.HasOne<Participant>().WithMany().HasForeignKey(x => new { x.GameId, x.GiverId }).OnDelete(DeleteBehavior.Restrict);
            e.HasOne<Participant>().WithMany().HasForeignKey(x => new { x.GameId, x.ReceiverId }).OnDelete(DeleteBehavior.Restrict);
            e.ToTable(t => t.HasCheckConstraint("no_self_gift", "\"GiverId\" <> \"ReceiverId\""));
        });
    }
}
