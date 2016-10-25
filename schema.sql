CREATE TABLE users
(
  id INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  uuid VARCHAR(36),
  username VARCHAR(100),
  email VARCHAR(100),
  created_at DATE,
  updated_at DATE
);


CREATE TABLE photos
(
  id INT(11) PRIMARY KEY NOT NULL AUTO_INCREMENT,
  user_id INT(11),
  uuid VARCHAR(36),
  url VARCHAR(500),
  likes INT(11) DEFAULT '0',
  expiry DATE,
  created_at DATE,
  updated_at DATE,
  CONSTRAINT photos_userid___fk FOREIGN KEY (user_id) REFERENCES users (id)
);
CREATE INDEX photos_userid___fk ON photos (user_id);