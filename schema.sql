-- sqlite3


CREATE TABLE `users` (
  `id`            INTEGER PRIMARY KEY AUTOINCREMENT,
  `uuid`          VARCHAR(36)  NULL,
  `username`      VARCHAR(100) NULL,
  `facebook_id`   VARCHAR(100) NULL,
  `profile_image` VARCHAR(100) NULL,
  `created_at`    DATE         NULL,
  `updated_at`    DATE         NULL
);

CREATE TABLE `user_tokens` (
  `uid`        INTEGER PRIMARY KEY,
  `token`      VARCHAR(64) NULL,
  `expiry`     DATE        NULL,
  `created_at` DATE        NULL,
  `updated_at` DATE        NULL
);