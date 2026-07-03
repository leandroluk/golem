CREATE TABLE IF NOT EXISTS "users" (
    "id" BIGSERIAL PRIMARY KEY,
    "name" VARCHAR(50) NOT NULL,
    "email" VARCHAR(50) NOT NULL
);

CREATE TABLE IF NOT EXISTS "post" (
    "id" BIGSERIAL PRIMARY KEY,
    "owner_user_id" BIGINT NOT NULL REFERENCES "users"("id"),
    "title" VARCHAR(50) NOT NULL,
    "content" TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS "category" (
    "id" BIGSERIAL PRIMARY KEY,
    "name" VARCHAR(50) NOT NULL
);

CREATE TABLE IF NOT EXISTS "post_to_category" (
    "post_id" BIGINT NOT NULL REFERENCES "post"("id"),
    "category_id" BIGINT NOT NULL REFERENCES "category"("id"),
    PRIMARY KEY ("post_id", "category_id")
);
