DROP TABLE IF EXISTS pumpkin;
CREATE TABLE pumpkin (
  id          INT AUTO_INCREMENT NOT NULL,
  name        VARCHAR(128) NOT NULL,
  description VARCHAR(128) NOT NULL,
  height      DECIMAL(6,2) NOT NULL,
  weight      DECIMAL(6,2) NOT NULL,
  PRIMARY KEY (`id`)
);

INSERT INTO pumpkin (name, description, height, weight)
VALUES
  ('Big Max', 'Giant pumpkin', 30.00, 200.00),
  ('Atlantic Giant', 'World record holder', 25.00, 150.00),
  ('Cinderella', 'French heirloom', 20.00, 100.00),
  ('Jack Be Little', 'Miniature pumpkin', 5.00, 2.00);