using Microsoft.EntityFrameworkCore.Migrations;

#nullable disable

namespace santa.Data.Migrations
{
    /// <inheritdoc />
    public partial class AddSantaMessages : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.AddColumn<string>(
                name: "SantaMessage",
                table: "Participants",
                type: "character varying(5000)",
                maxLength: 5000,
                nullable: false,
                defaultValue: "");
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropColumn(
                name: "SantaMessage",
                table: "Participants");
        }
    }
}
