-- MySQL dump 10.13  Distrib 5.7.16, for osx10.11 (x86_64)
--
-- Host: localhost    Database: stream_backend
-- ------------------------------------------------------
-- Server version	5.7.16

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;

--
-- Table structure for table `follows`
--

DROP TABLE IF EXISTS `follows`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `follows` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `user_id_1` int(11) DEFAULT NULL,
  `user_id_2` int(11) DEFAULT NULL,
  `uuid` varchar(40) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=16 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `follows`
--

LOCK TABLES `follows` WRITE;
/*!40000 ALTER TABLE `follows` DISABLE KEYS */;
INSERT INTO `follows` VALUES (4,1,2,NULL),(12,1,3,'ef7f4798-e63d-4747-b9d4-abc95dc1992e'),(13,3,2,'d389b9af-3fd3-49bb-9ced-a419aa778479'),(15,3,1,'215a6383-6d0a-4df3-87af-bb90d2c56bd8');
/*!40000 ALTER TABLE `follows` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `likes`
--

DROP TABLE IF EXISTS `likes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `likes` (
  `ID` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `CreatedAt` datetime DEFAULT NULL,
  `UpdatedAt` datetime DEFAULT NULL,
  `DeletedAt` datetime DEFAULT NULL,
  `UserID` int(10) unsigned DEFAULT NULL,
  `PhotoID` int(10) unsigned DEFAULT NULL,
  `FeedID` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`ID`)
) ENGINE=InnoDB AUTO_INCREMENT=12 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `likes`
--

LOCK TABLES `likes` WRITE;
/*!40000 ALTER TABLE `likes` DISABLE KEYS */;
INSERT INTO `likes` VALUES (4,NULL,NULL,NULL,2,6,'dd590632-a04e-11e6-8080-800041326adc'),(5,NULL,NULL,NULL,1,7,'050adf16-a04f-11e6-8080-800162152328'),(6,NULL,NULL,NULL,1,6,'97f2b486-a084-11e6-8080-800041326adc'),(7,NULL,NULL,NULL,2,7,'35a9325c-a0f0-11e6-8080-800162152328'),(8,NULL,NULL,NULL,3,5,'0871f88a-a1ab-11e6-8080-80004b330b13'),(9,NULL,NULL,NULL,3,4,'0ab54e76-a1ab-11e6-8080-800138858928'),(10,NULL,NULL,NULL,3,3,'0c331760-a1ab-11e6-8080-80006e86f4b4'),(11,NULL,NULL,NULL,3,7,'2f5262fa-a1ab-11e6-8080-800162152328');
/*!40000 ALTER TABLE `likes` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `photos`
--

DROP TABLE IF EXISTS `photos`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `photos` (
  `ID` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `CreatedAt` datetime DEFAULT NULL,
  `UpdatedAt` datetime DEFAULT NULL,
  `DeletedAt` datetime DEFAULT NULL,
  `UserID` int(11) DEFAULT NULL,
  `UUID` varchar(255) DEFAULT NULL,
  `URL` varchar(255) DEFAULT NULL,
  `likes` int(11) DEFAULT '0',
  PRIMARY KEY (`ID`)
) ENGINE=InnoDB AUTO_INCREMENT=9 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `photos`
--

LOCK TABLES `photos` WRITE;
/*!40000 ALTER TABLE `photos` DISABLE KEYS */;
INSERT INTO `photos` VALUES (3,'2016-11-01 17:06:32','2016-11-01 17:06:33',NULL,2,'fcd1b361-74c0-4fa5-84d3-25966f3435e0','https://android-demo.s3.amazonaws.com/photos/476ac193-8f03-425c-81ca-04d7c55bd9fd.png',0),(4,'2016-11-01 17:06:50','2016-11-01 17:06:51',NULL,1,'64f3c633-dd3c-404d-ab81-34cf00e89917','https://android-demo.s3.amazonaws.com/photos/d326223c-5a01-40a1-a419-dd68292be6c1.png',0),(5,'2016-11-01 17:22:14','2016-11-01 17:22:15',NULL,2,'9036ae14-3d13-44f8-a82d-d60df233c0b6','https://android-demo.s3.amazonaws.com/photos/1ddf18ce-9ee1-4863-9e24-aa1a30da942e.png',0),(6,'2016-11-01 19:50:25','2016-11-01 19:50:27',NULL,3,'bf459b22-41b5-43a2-9278-567894c2b011','https://android-demo.s3.amazonaws.com/photos/82a386ea-c07a-4469-9c09-e015129669d3.png',0),(7,'2016-11-01 22:19:30','2016-11-01 22:19:32',NULL,3,'66998f98-af10-48ff-97a0-891154584306','https://android-demo.s3.amazonaws.com/photos/3a98b339-7e81-46f0-9692-68ca9ac19ac6.png',0),(8,'2016-11-04 15:19:42','2016-11-04 15:19:43',NULL,2,'4b3db6be-16b4-4704-a300-2cfc278491cc','https://android-demo.s3.amazonaws.com/photos/ac82481a-d111-4c93-8b4d-bb68faf2031d.png',0);
/*!40000 ALTER TABLE `photos` ENABLE KEYS */;
UNLOCK TABLES;

--
-- Table structure for table `users`
--

DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8 */;
CREATE TABLE `users` (
  `ID` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `CreatedAt` datetime DEFAULT NULL,
  `UpdatedAt` datetime DEFAULT NULL,
  `DeletedAt` datetime DEFAULT NULL,
  `UUID` varchar(255) DEFAULT NULL,
  `Username` varchar(255) DEFAULT NULL,
  `Email` varchar(255) DEFAULT NULL,
  PRIMARY KEY (`ID`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping data for table `users`
--

LOCK TABLES `users` WRITE;
/*!40000 ALTER TABLE `users` DISABLE KEYS */;
INSERT INTO `users` VALUES (1,'2016-10-25 06:11:51','2016-10-25 06:11:51',NULL,'9cf34d34-a042-4231-babc-eee6ba67bd18','ian','ian@getstream.io'),(2,'2016-10-26 17:33:35','2016-10-26 17:33:35',NULL,'03a1cfed-3590-4aa8-a592-f78bc71ccfbd','josh','josh@getstream.io'),(3,'2016-11-01 19:49:27','2016-11-01 19:49:27',NULL,'7eadc152-dea3-44d2-b0f5-d7fbf94e5a15','nick','nick@getstream.io');
/*!40000 ALTER TABLE `users` ENABLE KEYS */;
UNLOCK TABLES;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;

-- Dump completed on 2016-11-04  9:37:32
