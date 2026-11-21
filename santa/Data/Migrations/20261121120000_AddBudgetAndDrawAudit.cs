using System;
using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable
namespace santa.Data.Migrations;
public partial class AddBudgetAndDrawAudit : Migration
{
    protected override void Up(MigrationBuilder migrationBuilder)
    {
        migrationBuilder.AddColumn<long>("BudgetMinor", "Games", nullable: false, defaultValue: 0L);
        migrationBuilder.AddColumn<string>("Currency", "Games", maxLength: 3, nullable: false, defaultValue: "EUR");
        migrationBuilder.AddColumn<string>("BudgetNote", "Games", maxLength: 500, nullable: false, defaultValue: "");
        migrationBuilder.AddCheckConstraint("nonnegative_budget", "Games", "\"BudgetMinor\" >= 0");
        migrationBuilder.AddColumn<string>("InputDigest", "Draws", maxLength: 64, nullable: false, defaultValue: "");
        migrationBuilder.AddColumn<string>("ResultDigest", "Draws", maxLength: 64, nullable: false, defaultValue: "");
        migrationBuilder.AddColumn<int>("ParticipantCount", "Draws", nullable: false, defaultValue: 0);
        migrationBuilder.AddColumn<int>("ExclusionCount", "Draws", nullable: false, defaultValue: 0);
        migrationBuilder.AddColumn<DateTimeOffset>("PublishedAt", "Draws", nullable: false, defaultValue: DateTimeOffset.UnixEpoch);
    }
    protected override void Down(MigrationBuilder migrationBuilder)
    {
        migrationBuilder.DropCheckConstraint("nonnegative_budget", "Games");
        foreach (var c in new[] { "BudgetMinor", "Currency", "BudgetNote" }) migrationBuilder.DropColumn(c, "Games");
        foreach (var c in new[] { "InputDigest", "ResultDigest", "ParticipantCount", "ExclusionCount", "PublishedAt" }) migrationBuilder.DropColumn(c, "Draws");
    }
}
