-- =====================================================================
--  SeduhPoin - Migrasi Sistem Login & Role
--
--  Untuk database yang SUDAH ADA dan sudah berisi data.
--  migrate.sql TIDAK menghapus apa pun (berbeda dengan schema.sql yang
--  memakai DROP TABLE). File ini aman dijalankan berulang kali.
--
--  CARA PAKAI:
--    1. Backup dulu:  mysqldump -u root seduhpoin > backup.sql
--    2. Jalankan:     mysql -u root < migrate.sql
--    3. Jalankan aplikasi.
--
--  Perintah yang dipakai: mysql -u root -p seduhpoin < migrate.sql
--
--  CATATAN: MySQL tidak punya "ADD COLUMN IF NOT EXISTS", jadi penambahan
--  kolom dicek lewat information_schema lalu statement-nya disiapkan
--  dengan PREPARE/EXECUTE. Pola yang sama dipakai untuk foreign key.
-- =====================================================================

CREATE DATABASE IF NOT EXISTS seduhpoin
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;

USE seduhpoin;

-- ---------------------------------------------------------------------
-- 1. Tabel pengguna (akun staf)
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS pengguna (
  id                INT UNSIGNED NOT NULL AUTO_INCREMENT,
  username          VARCHAR(30)  NOT NULL,
  nama              VARCHAR(100) NOT NULL,
  password_hash     VARCHAR(255) NOT NULL,
  role              ENUM('ADMIN','KASIR') NOT NULL DEFAULT 'KASIR',
  aktif             TINYINT(1)   NOT NULL DEFAULT 1,
  harus_ganti_sandi TINYINT(1)   NOT NULL DEFAULT 0,
  created_at        DATETIME     NOT NULL,
  last_login_at     DATETIME     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_pengguna_username (username),
  KEY idx_pengguna_role (role, aktif)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 2. Tabel sesi
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sesi (
  id             INT UNSIGNED NOT NULL AUTO_INCREMENT,
  token_hash     CHAR(64)     NOT NULL,
  jenis          ENUM('PENGGUNA','MEMBER') NOT NULL,
  pengguna_id    INT UNSIGNED NULL,
  member_id      INT UNSIGNED NULL,
  peran          VARCHAR(20)  NOT NULL,
  dibuat_at      DATETIME     NOT NULL,
  kedaluwarsa_at DATETIME     NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_sesi_token (token_hash),
  KEY idx_sesi_kedaluwarsa (kedaluwarsa_at),
  KEY idx_sesi_pengguna (pengguna_id),
  KEY idx_sesi_member (member_id),
  CONSTRAINT fk_sesi_pengguna FOREIGN KEY (pengguna_id) REFERENCES pengguna(id) ON DELETE CASCADE,
  CONSTRAINT fk_sesi_member   FOREIGN KEY (member_id)   REFERENCES member(id)   ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 3. Kolom baru pada tabel member: password_hash
--
--    NULL = member belum punya akun login portal.
-- ---------------------------------------------------------------------
SET @sql := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE member ADD COLUMN password_hash VARCHAR(255) NULL AFTER nama',
  'DO 0')
  FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'seduhpoin' AND TABLE_NAME = 'member' AND COLUMN_NAME = 'password_hash');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ---------------------------------------------------------------------
-- 4. Kolom baru pada tabel transaksi: pengguna_id dan kasir_nama
-- ---------------------------------------------------------------------
SET @sql := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE transaksi ADD COLUMN pengguna_id INT UNSIGNED NULL AFTER member_id',
  'DO 0')
  FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'seduhpoin' AND TABLE_NAME = 'transaksi' AND COLUMN_NAME = 'pengguna_id');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE transaksi ADD COLUMN kasir_nama VARCHAR(100) NULL AFTER pengguna_id',
  'DO 0')
  FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = 'seduhpoin' AND TABLE_NAME = 'transaksi' AND COLUMN_NAME = 'kasir_nama');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Foreign key transaksi -> pengguna (ON DELETE SET NULL, transaksi lama tetap utuh).
SET @sql := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE transaksi ADD CONSTRAINT fk_transaksi_pengguna FOREIGN KEY (pengguna_id) REFERENCES pengguna(id) ON DELETE SET NULL',
  'DO 0')
  FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA = 'seduhpoin' AND TABLE_NAME = 'transaksi'
    AND CONSTRAINT_NAME = 'fk_transaksi_pengguna');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ---------------------------------------------------------------------
-- 5. Akun demo
--
--    Hash bcrypt di bawah dibuat dengan: go run ./cmd/hashkata <sandi>
--    Akun admin WAJIB mengganti kata sandi pada login pertama.
-- ---------------------------------------------------------------------
INSERT IGNORE INTO pengguna
  (username, nama, password_hash, role, aktif, harus_ganti_sandi, created_at)
VALUES
  ('admin', 'Administrator', '$2a$10$SmHm3F8f1uwbkLEqCNvZQ.S3/QrR4jZq5JXmT/6CHZ11o78kOYYoi', 'ADMIN', 1, 1, '2026-01-01 00:00:00'),
  ('kasir', 'Kasir Demo',    '$2a$10$xnyLuVXZBIrrxG25Xluz0OxsjIUfUqZOu5wdVXrXmh61vK8pH0jxO', 'KASIR', 1, 0, '2026-01-01 00:00:00');

-- Member yang sudah terdaftar diberi akun login portal ("member123").
-- Baris yang sudah punya password_hash tidak ditimpa, supaya sandi yang
-- sudah diganti member tidak hilang saat migrasi dijalankan ulang.
UPDATE member
SET password_hash = '$2a$10$If/YiycbdbxKG8J/3TJxRehIjQVqGiH7rn0nKC0VK76tysWQb7wWi'
WHERE no_hp IN ('081234567890', '081298765432', '085611223344')
  AND password_hash IS NULL;

-- =====================================================================
--  Selesai. Verifikasi:
--    SHOW TABLES;                                    -- pengguna, sesi ada
--    SHOW COLUMNS FROM member LIKE 'password_hash';
--    SHOW COLUMNS FROM transaksi LIKE 'kasir_nama';
--    SELECT username, role, aktif, harus_ganti_sandi FROM pengguna;
-- =====================================================================
