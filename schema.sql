CREATE TABLE users
(
  ID INT(10) unsigned PRIMARY KEY NOT NULL AUTO_INCREMENT,
  CreatedAt DATETIME,
  UpdatedAt DATETIME,
  DeletedAt DATETIME,
  UUID VARCHAR(255),
  Username VARCHAR(255),
  Email VARCHAR(255)
);
INSERT INTO users (ID, CreatedAt, UpdatedAt, DeletedAt, UUID, Username, Email) VALUES (1, '2016-10-25 06:11:51', '2016-10-25 06:11:51', null, '9cf34d34-a042-4231-babc-eee6ba67bd18', 'ian', 'ian@example.com');
INSERT INTO users (ID, CreatedAt, UpdatedAt, DeletedAt, UUID, Username, Email) VALUES (2, '2016-10-26 17:33:35', '2016-10-26 17:33:35', null, '03a1cfed-3590-4aa8-a592-f78bc71ccfbd', 'josh', 'josh@getstream.io');


CREATE TABLE photos
(
  ID INT(10) unsigned PRIMARY KEY NOT NULL AUTO_INCREMENT,
  CreatedAt DATETIME,
  UpdatedAt DATETIME,
  DeletedAt DATETIME,
  UserID INT(11),
  UUID VARCHAR(255),
  URL VARCHAR(255),
  Likes INT(11)
);
INSERT INTO photos (CreatedAt, UpdatedAt, DeletedAt, UserID, UUID, URL, Likes) VALUES ('2016-10-25 18:28:28', '2016-10-25 18:28:29', null, 1, '3c7c77bd-e1b4-4e64-9c9d-fff223efc17b', 'https://android-demo.s3.amazonaws.com/photos/f5222729-17d5-4b21-bade-a3e7ce1adb1c.png', null);


CREATE TABLE follows
(
  id INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  user_id_1 INT(11),
  user_id_2 INT(11)
);
INSERT INTO follows (user_id_1, user_id_2) VALUES (1, 2);
