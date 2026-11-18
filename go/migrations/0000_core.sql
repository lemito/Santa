CREATE TABLE IF NOT EXISTS "AspNetUsers" (
 "Id" uuid PRIMARY KEY,
 "DisplayName" varchar(100) NOT NULL,
 "UserName" varchar(256),
 "NormalizedUserName" varchar(256),
 "Email" varchar(256),
 "NormalizedEmail" varchar(256),
 "EmailConfirmed" boolean NOT NULL DEFAULT false,
 "PasswordHash" text,
 "SecurityStamp" text,
 "ConcurrencyStamp" text,
 "PhoneNumber" text,
 "PhoneNumberConfirmed" boolean NOT NULL DEFAULT false,
 "TwoFactorEnabled" boolean NOT NULL DEFAULT false,
 "LockoutEnd" timestamptz,
 "LockoutEnabled" boolean NOT NULL DEFAULT true,
 "AccessFailedCount" integer NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS "UserNameIndex" ON "AspNetUsers" ("NormalizedUserName");
CREATE INDEX IF NOT EXISTS "EmailIndex" ON "AspNetUsers" ("NormalizedEmail");

CREATE TABLE IF NOT EXISTS "Games" (
 "Id" uuid PRIMARY KEY,
 "OwnerId" uuid NOT NULL REFERENCES "AspNetUsers"("Id") ON DELETE RESTRICT,
 "Title" varchar(150) NOT NULL,
 "State" varchar(16) NOT NULL CHECK ("State" IN ('Draft','Published','Closed')),
 "Version" bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS "IX_Games_OwnerId" ON "Games" ("OwnerId");

CREATE TABLE IF NOT EXISTS "Participants" (
 "GameId" uuid NOT NULL REFERENCES "Games"("Id") ON DELETE CASCADE,
 "AccountId" uuid NOT NULL REFERENCES "AspNetUsers"("Id") ON DELETE RESTRICT,
 "SantaMessage" varchar(5000) NOT NULL DEFAULT '',
 PRIMARY KEY ("GameId", "AccountId")
);
ALTER TABLE "Participants" ADD COLUMN IF NOT EXISTS "SantaMessage" varchar(5000) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS "IX_Participants_AccountId" ON "Participants" ("AccountId");

CREATE TABLE IF NOT EXISTS "Exclusions" (
 "GameId" uuid NOT NULL,
 "FirstId" uuid NOT NULL,
 "SecondId" uuid NOT NULL,
 PRIMARY KEY ("GameId", "FirstId", "SecondId"),
 FOREIGN KEY ("GameId", "FirstId") REFERENCES "Participants"("GameId", "AccountId") ON DELETE CASCADE,
 FOREIGN KEY ("GameId", "SecondId") REFERENCES "Participants"("GameId", "AccountId") ON DELETE CASCADE,
 CHECK ("FirstId" <> "SecondId")
);

CREATE TABLE IF NOT EXISTS "Draws" (
 "GameId" uuid PRIMARY KEY REFERENCES "Games"("Id") ON DELETE RESTRICT,
 "Version" bigint NOT NULL,
 "RequestKey" uuid NOT NULL,
 "EngineVersion" text NOT NULL
);

CREATE TABLE IF NOT EXISTS "Assignments" (
 "GameId" uuid NOT NULL REFERENCES "Draws"("GameId") ON DELETE CASCADE,
 "GiverId" uuid NOT NULL,
 "ReceiverId" uuid NOT NULL,
 "RecipientMessage" varchar(5000) NOT NULL DEFAULT '',
 PRIMARY KEY ("GameId", "GiverId"),
 FOREIGN KEY ("GameId", "GiverId") REFERENCES "Participants"("GameId", "AccountId") ON DELETE RESTRICT,
 FOREIGN KEY ("GameId", "ReceiverId") REFERENCES "Participants"("GameId", "AccountId") ON DELETE RESTRICT,
 CHECK ("GiverId" <> "ReceiverId")
);
ALTER TABLE "Assignments" ADD COLUMN IF NOT EXISTS "RecipientMessage" varchar(5000) NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS "IX_Assignments_GameId_ReceiverId" ON "Assignments" ("GameId", "ReceiverId");
