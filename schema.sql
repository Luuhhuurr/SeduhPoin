-- =====================================================================
--  SeduhPoin - Skema Basis Data + Seed Awal
--  Kedai Kopi Seduh (POS + Loyalty Program)
--
--  Import lewat phpMyAdmin (XAMPP) atau: mysql -u root < schema.sql
--
--  CATATAN PENTING UNTUK SIDANG:
--   * Semua tabel memakai engine InnoDB, karena pembatalan transaksi dan
--     simpan transaksi memakai sql.Tx (transaksi database), dan InnoDB
--     mendukung foreign key + row locking (SELECT ... FOR UPDATE).
--   * Semua kolom nominal (uang) dan poin bertipe BIGINT -> di Go dibungkus
--     int64. Tidak ada float untuk nilai uang; float hanya dipakai untuk
--     pengali tingkat (1.5 untuk Gold).
--   * Semua kolom tanggal/waktu diisi dengan waktu aplikasi dalam zona
--     WIB (Asia/Jakarta), bukan NOW() server, agar konsisten dengan
--     tampilan dan perhitungan masa berlaku poin.
-- =====================================================================

CREATE DATABASE IF NOT EXISTS seduhpoin
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;

USE seduhpoin;

-- Foreign key checking dinonaktifkan sementara agar tabel boleh di-drop
-- dalam urutan bebas (dari tabel yang bergantung ke tabel paling dasar).
SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS mutasi_poin;
DROP TABLE IF EXISTS poin_lot;
DROP TABLE IF EXISTS transaksi_promo;
DROP TABLE IF EXISTS transaksi_item;
DROP TABLE IF EXISTS transaksi;
DROP TABLE IF EXISTS voucher;
DROP TABLE IF EXISTS promo_kategori;
DROP TABLE IF EXISTS promo_produk;
DROP TABLE IF EXISTS promo;
DROP TABLE IF EXISTS member;
DROP TABLE IF EXISTS produk;
DROP TABLE IF EXISTS cabang;

SET FOREIGN_KEY_CHECKS = 1;

-- ---------------------------------------------------------------------
-- 1. Cabang
--
-- Sumber data transaksi (tiap transaksi selalu punya satu cabang).
-- Soft delete: baris tidak pernah dihapus, hanya diisi deleted_at.
-- Transaksi lama tetap menunjuk ke cabang yang benar walau cabang
-- kemudian di-nonaktifkan.
-- ---------------------------------------------------------------------
CREATE TABLE cabang (
  id        INT UNSIGNED NOT NULL AUTO_INCREMENT,
  kode      VARCHAR(20)  NOT NULL,
  nama      VARCHAR(100) NOT NULL,
  alamat    VARCHAR(255) NOT NULL DEFAULT '',
  telepon   VARCHAR(30)  NOT NULL DEFAULT '',
  deleted_at DATETIME     NULL,          -- non-NULL = sudah dihapus (soft delete)
  PRIMARY KEY (id),
  UNIQUE KEY uq_cabang_kode (kode),      -- kode cabang unik
  KEY idx_cabang_deleted (deleted_at)    -- index untuk filter "belum dihapus"
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 2. Produk
--
-- Harga disimpan dalam rupiah penuh sebagai BIGINT (contoh: 28000 = Rp28.000).
-- Kolom kategori dipakai promo B2G1: setiap 3 unit dalam satu kategori
-- memberi 1 unit termurah gratis, dan unit digabung per kategori.
-- ---------------------------------------------------------------------
CREATE TABLE produk (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  kode       VARCHAR(20)  NOT NULL,
  nama       VARCHAR(100) NOT NULL,
  kategori   VARCHAR(50)  NOT NULL,      -- Kopi / Minuman / Makanan / Retail
  harga      BIGINT       NOT NULL,      -- rupiah penuh, selalu > 0
  deleted_at DATETIME     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_produk_kode (kode),
  KEY idx_produk_kategori (kategori),   -- mempercepat query promo B2G1
  KEY idx_produk_deleted (deleted_at),
  CONSTRAINT ck_produk_harga CHECK (harga > 0)   -- harga harus positif
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 3. Member (pelanggan terdaftar pemilik loyalty)
--
-- Hanya member tabel ini yang memperoleh dan menukar poin.
-- Pelanggan non-member tetap boleh memakai promo, tetapi tidak memiliki poin.
--
-- PENTING: tabel ini TIDAK punya kolom tingkat (Silver/Gold) maupun saldo
-- poin. Keduanya dihitung on-the-fly saat aplikasi berjalan:
--   - tingkat  : dari total belanja 3 bulan terakhir (transaksi SELESAI),
--                ambang Rp1.500.000 = GOLD, selain itu SILVER;
--   - saldo    : jumlah_sisa lot poin yang masih AKTIF dan belum kedaluwarsa.
-- Tujuannya agar data member tidak perlu "dipelihara" oleh job terjadwal.
--
-- no_hp wajib unik dan hanya berisi digit: dipakai untuk mencari member
-- pada form transaksi.
-- ---------------------------------------------------------------------
CREATE TABLE member (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  no_hp      VARCHAR(20)  NOT NULL,      -- unik, hanya digit
  nama       VARCHAR(100) NOT NULL,
  tgl_daftar DATE         NOT NULL,
  deleted_at DATETIME     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_member_no_hp (no_hp),
  KEY idx_member_deleted (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 4. Promo (4 jenis: BR-09)
--
-- Satu tabel untuk keempat jenis promo, dibedakan oleh kolom `jenis`:
--   DISKON_PRODUK        : diskon persen untuk produk tertentu (detail di
--                          tabel promo_produk)
--   POTONGAN_MIN_BELANJA : potongan nominal bila belanja >= min_belanja
--   B2G1                 : beli 2 gratis 1 untuk kategori tertentu (detail di
--                          tabel promo_kategori)
--   VOUCHER              : ber kode, ber kuota (detail di tabel voucher)
--
-- Pengaturan berlaku:
--   * tgl_mulai/tgl_selesai = masa berlaku promo.
--   * jam_mulai/jam_selesai = rentang jam; NULL berarti sepanjang hari.
--   * cabang_ids             = daftar id cabang berformat CSV ("1,2,3");
--                              NULL/kosong berarti berlaku untuk semua cabang.
--   * aktif                  = 0 berarti promo dinonaktifkan.
--
-- Promo yang sudah pernah dipakai TIDAK BOLEH dihapus, hanya dinonaktifkan
-- (dicek di handler promo). Baris promo tidak pernah dihapus permanen.
-- ---------------------------------------------------------------------
CREATE TABLE promo (
  id            INT UNSIGNED NOT NULL AUTO_INCREMENT,
  kode          VARCHAR(30)  NOT NULL,
  nama          VARCHAR(100) NOT NULL,
  jenis         ENUM('DISKON_PRODUK','POTONGAN_MIN_BELANJA','B2G1','VOUCHER') NOT NULL,
  nilai_persen  INT UNSIGNED NULL,              -- diskon % produk tertentu
  nilai_nominal BIGINT       NULL,              -- potongan nominal / nilai voucher
  min_belanja   BIGINT       NULL,              -- syarat minimal belanja
  tgl_mulai     DATE         NOT NULL,
  tgl_selesai   DATE         NOT NULL,
  jam_mulai     TIME         NULL,              -- kosong = sepanjang hari
  jam_selesai   TIME         NULL,
  cabang_ids    VARCHAR(255) NULL,              -- CSV id cabang; kosong = semua cabang
  aktif         TINYINT(1)   NOT NULL DEFAULT 1,
  deleted_at    DATETIME     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_promo_kode (kode),
  KEY idx_promo_jenis_aktif (jenis, aktif),    -- filter promo yang bisa dipakai
  CONSTRAINT ck_promo_periode CHECK (tgl_selesai >= tgl_mulai)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Produk yang mendapat diskon % (hanya jenis DISKON_PRODUK).
-- Relasi many-to-many: satu promo bisa berlaku untuk banyak produk.
CREATE TABLE promo_produk (
  promo_id INT UNSIGNED NOT NULL,
  produk_id INT UNSIGNED NOT NULL,
  persen   INT UNSIGNED NOT NULL,      -- 1..100
  PRIMARY KEY (promo_id, produk_id),
  KEY idx_pp_produk (produk_id),
  CONSTRAINT fk_pp_promo  FOREIGN KEY (promo_id) REFERENCES promo(id)  ON DELETE CASCADE,
  CONSTRAINT fk_pp_produk FOREIGN KEY (produk_id) REFERENCES produk(id) ON DELETE CASCADE,
  CONSTRAINT ck_pp_persen CHECK (persen > 0 AND persen <= 100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Kategori yang mendapat promo B2G1 (hanya jenis B2G1).
-- Unit produk dalam kategori yang sama digabung bersama saat menghitung
-- "setiap 3 unit -> 1 unit termurah gratis".
CREATE TABLE promo_kategori (
  promo_id INT UNSIGNED NOT NULL,
  kategori VARCHAR(50) NOT NULL,
  PRIMARY KEY (promo_id, kategori),
  CONSTRAINT fk_pk_promo FOREIGN KEY (promo_id) REFERENCES promo(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 5. Voucher (berkode, berkuota)
--
-- Maksimal 1 voucher per transaksi. Kode voucher unik.
--
-- Keamanan kuota dijamin oleh database DAN oleh query:
--   UPDATE voucher SET terpakai = terpakai + 1
--    WHERE kode = ? AND terpakai < kuota;
-- lalu jumlah baris yang terpengaruh diperiksa. Dengan begitu kuota
-- tidak mungkin terlampaui walaupun ada dua kasir memproses bersamaan.
-- CHECK di bawah adalah pengaman kedua di sisi database.
-- ---------------------------------------------------------------------
CREATE TABLE voucher (
  kode      VARCHAR(30)  NOT NULL,
  promo_id  INT UNSIGNED NOT NULL,
  kuota     INT UNSIGNED NOT NULL DEFAULT 0,
  terpakai  INT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (kode),
  KEY idx_voucher_promo (promo_id),
  CONSTRAINT fk_voucher_promo FOREIGN KEY (promo_id) REFERENCES promo(id) ON DELETE CASCADE,
  CONSTRAINT ck_voucher_kuota CHECK (terpakai <= kuota)   -- kuota tak pernah < terpakai
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 6. Transaksi
--
-- Header transaksi. Semua nominal di sini adalah hasil perhitungan di
-- package internal/calc (tidak dihitung ulang di SQL).
--
-- tingkat_saat_transaksi & pengali_saat_transaksi:
--   Menyimpan tingkat member yang berlaku SAAT transaksi dimulai, sehingga
--   hasil perolehan poin tidak berubah retroactive ketika member naik
--   tingkat di kemudian hari.
--
-- member_id boleh NULL: transaksi non-member tetap tercatat (dan tetap
-- bisa memakai promo), hanya tidak memperoleh/menukar poin.
--
-- status: SELESAI = aktif dan dihitung laporan; BATAL = dibatalkan.
-- Transaksi BATAL TIDAK dihapus, hanya ditandai, sehingga riwayat tetap
-- terbaca dan biaya promonya keluar dari laporan.
-- ---------------------------------------------------------------------
CREATE TABLE transaksi (
  id                     INT UNSIGNED NOT NULL AUTO_INCREMENT,
  kode                   VARCHAR(30)  NOT NULL,
  cabang_id              INT UNSIGNED NOT NULL,
  member_id              INT UNSIGNED NULL,
  tanggal_transaksi      DATETIME     NOT NULL,
  tingkat_saat_transaksi ENUM('SILVER','GOLD') NOT NULL DEFAULT 'SILVER',
  pengali_saat_transaksi DECIMAL(3,2) NOT NULL DEFAULT 1.00,
  subtotal               BIGINT       NOT NULL DEFAULT 0,
  total_setelah_promo    BIGINT       NOT NULL DEFAULT 0,
  poin_ditukar           BIGINT       NOT NULL DEFAULT 0,
  total_bayar            BIGINT       NOT NULL DEFAULT 0,
  voucher_kode           VARCHAR(30)  NULL,   -- disimpan agar kuota bisa dikembalikan saat pembatalan
  status                 ENUM('SELESAI','BATAL') NOT NULL DEFAULT 'SELESAI',
  created_at             DATETIME     NOT NULL,
  batal_at               DATETIME     NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_transaksi_kode (kode),
  KEY idx_transaksi_cabang_tgl (cabang_id, tanggal_transaksi),  -- filter riwayat per cabang & tanggal
  KEY idx_transaksi_member (member_id),                        -- hitung belanja 3 bulan untuk tingkat
  KEY idx_transaksi_status (status),                           -- laporan hanya menghitung SELESAI
  CONSTRAINT fk_transaksi_cabang FOREIGN KEY (cabang_id) REFERENCES cabang(id),
  CONSTRAINT fk_transaksi_member FOREIGN KEY (member_id) REFERENCES member(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Rincian item. nama_produk, kategori, dan harga DISALIN ke sini saat
-- transaksi disimpan, agar transaksi lama tetap menampilkan nama & harga
-- lama walaupun produk master berubah atau di-soft-delete.
CREATE TABLE transaksi_item (
  id             INT UNSIGNED NOT NULL AUTO_INCREMENT,
  transaksi_id   INT UNSIGNED NOT NULL,
  produk_id      INT UNSIGNED NULL,
  nama_produk    VARCHAR(100) NOT NULL,
  kategori       VARCHAR(50)  NOT NULL DEFAULT '',
  harga          BIGINT       NOT NULL,
  jumlah         INT UNSIGNED NOT NULL,
  subtotal       BIGINT       NOT NULL,
  persen_diskon  INT UNSIGNED NOT NULL DEFAULT 0,
  potongan       BIGINT       NOT NULL DEFAULT 0,   -- total potongan B2G1 + diskon % pada baris ini
  PRIMARY KEY (id),
  KEY idx_ti_transaksi (transaksi_id),
  KEY idx_ti_produk (produk_id),
  CONSTRAINT fk_ti_transaksi FOREIGN KEY (transaksi_id) REFERENCES transaksi(id) ON DELETE CASCADE,
  CONSTRAINT fk_ti_produk    FOREIGN KEY (produk_id) REFERENCES produk(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Rincian potongan per promo pada sebuah transaksi.
-- Tabel inilah sumber data laporan biaya promo (FR-18): satu baris per
-- promo per transaksi, berisi nilai potongan rupiah yang dihitung aplikasi.
CREATE TABLE transaksi_promo (
  id             INT UNSIGNED NOT NULL AUTO_INCREMENT,
  transaksi_id   INT UNSIGNED NOT NULL,
  promo_id       INT UNSIGNED NULL,
  nama_promo     VARCHAR(100) NOT NULL,
  jenis          VARCHAR(30)  NOT NULL,
  nilai_potongan BIGINT       NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_tp_transaksi (transaksi_id),
  KEY idx_tp_promo (promo_id),
  CONSTRAINT fk_tp_transaksi FOREIGN KEY (transaksi_id) REFERENCES transaksi(id) ON DELETE CASCADE,
  CONSTRAINT fk_tp_promo     FOREIGN KEY (promo_id) REFERENCES promo(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- ---------------------------------------------------------------------
-- 7. Lot poin (FIFO) & mutasi poin
--
-- MODEL DATA POIN
-- Poin tidak disimpan sebagai satu angka "saldo" di tabel member, karena
-- setiap poin punya masa berlaku sendiri (6 bulan sejak diperoleh).
-- Poin yang diperoleh disimpan sebagai satu LOT: baris di tabel poin_lot.
--
-- FIFO: saat menukar poin, lot dengan tanggal_kadaluarsa TERDEKAT dipakai
-- lebih dulu; bila sama, yang diperoleh lebih dulu didahulukan. Lot yang
-- dikunci dengan SELECT ... FOR UPDATE supaya tidak dipakai dua kali
-- oleh dua transaksi bersamaan.
--
-- jumlah_awal  : jumlah poin saat lot dibuat
-- jumlah_sisa   : jumlah poin yang masih tersisa pada lot tersebut
-- status        : AKTIF = masih bisa dipakai, DITARIK = withdrawn karena
--                 transaksi asalnya dibatalkan.
--
-- Saldo aktif member = SUM(jumlah_sisa) untuk lot berstatus AKTIF yang
-- tanggal_kadaluarsa >= hari ini.
-- ---------------------------------------------------------------------
CREATE TABLE poin_lot (
  id                 INT UNSIGNED NOT NULL AUTO_INCREMENT,
  member_id          INT UNSIGNED NOT NULL,
  transaksi_id       INT UNSIGNED NULL,
  jumlah_awal        INT UNSIGNED NOT NULL,
  jumlah_sisa        INT UNSIGNED NOT NULL,
  tanggal_dapat      DATE         NOT NULL,
  tanggal_kadaluarsa DATE         NOT NULL,   -- tanggal_dapat + 6 bulan
  status             ENUM('AKTIF','DITARIK') NOT NULL DEFAULT 'AKTIF',
  PRIMARY KEY (id),
  KEY idx_pl_member_status_kadaluarsa (member_id, status, tanggal_kadaluarsa),  -- index FIFO + saldo
  KEY idx_pl_transaksi (transaksi_id),
  CONSTRAINT fk_pl_member    FOREIGN KEY (member_id)    REFERENCES member(id),
  CONSTRAINT fk_pl_transaksi FOREIGN KEY (transaksi_id) REFERENCES transaksi(id) ON DELETE CASCADE,
  CONSTRAINT ck_pl_sisa CHECK (jumlah_sisa <= jumlah_awal)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Mutasi poin = riwayat/bookkeeping setiap perubahan saldo.
-- Tipe: PEROLEHAN (dari transaksi), PENUKARAN (ditukar member),
--       KEMBALI (dikembalikan saat pembatalan), TARIK (ditarik saat pembatalan).
-- Tabel ini adalah "bukti" yang bisa ditunjukkan saat pembatalan ditolak
-- karena poin perolehan transaksi tersebut sudah dipakai di transaksi lain.
CREATE TABLE mutasi_poin (
  id           INT UNSIGNED NOT NULL AUTO_INCREMENT,
  member_id    INT UNSIGNED NOT NULL,
  tipe         ENUM('PEROLEHAN','PENUKARAN','KEMBALI','TARIK') NOT NULL,
  jumlah       INT UNSIGNED NOT NULL,
  lot_id       INT UNSIGNED NULL,      -- lot yang terpengaruh (dikunci FOR UPDATE)
  transaksi_id INT UNSIGNED NULL,
  keterangan   VARCHAR(255) NOT NULL DEFAULT '',
  created_at   DATETIME     NOT NULL,
  PRIMARY KEY (id),
  KEY idx_mp_member (member_id),
  KEY idx_mp_transaksi (transaksi_id),
  CONSTRAINT fk_mp_member    FOREIGN KEY (member_id)    REFERENCES member(id),
  CONSTRAINT fk_mp_lot       FOREIGN KEY (lot_id)       REFERENCES poin_lot(id) ON DELETE SET NULL,
  CONSTRAINT fk_mp_transaksi FOREIGN KEY (transaksi_id) REFERENCES transaksi(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =====================================================================
--  SEED DATA AWAL
--
--  Data contoh agar aplikasi langsung bisa dicoba tanpa input manual.
--  Hapus baris DROP TABLE di atas bila ingin mengulang dari nol.
-- =====================================================================

-- 3 Cabang
INSERT INTO cabang (kode, nama, alamat, telepon) VALUES
  ('C01', 'SeduhPoin Pusat',  'Jl. Merdeka No. 10, Jakarta',  '021-5550101'),
  ('C02', 'SeduhPoin Sudirman','Jl. Jend. Sudirman No. 25, Jakarta', '021-5550102'),
  ('C03', 'SeduhPoin Bandung','Jl. Asia Afrika No. 8, Bandung',   '022-5550103');

-- 12 Produk, 4 kategori (Kopi, Minuman, Makanan, Retail)
INSERT INTO produk (kode, nama, kategori, harga) VALUES
  ('P01', 'Kopi Susu Gula Aren',   'Kopi',     28000),
  ('P02', 'Kopi Susu Regular',    'Kopi',     22000),
  ('P03', 'Americano',            'Kopi',     20000),
  ('P04', 'Espresso-single',      'Kopi',     18000),
  ('P05', 'Matcha Latte',         'Minuman',  26000),
  ('P06', 'Chocolate Latte',      'Minuman',  27000),
  ('P07', 'Teh Lemon',            'Minuman',  15000),
  ('P08', 'Croissant Mentega',    'Makanan',  20000),
  ('P09', 'Nasi Goreng Spesial',  'Makanan',  35000),
  ('P10', 'Roti Bakar Cokelat',   'Makanan',  18000),
  ('P11', 'Biji Kopi 250g',       'Retail',   95000),
  ('P12', 'Gula Aren Botol 250ml','Retail',   35000);

-- 3 Member contoh. Nomor HP inilah yang dipakai untuk mencari member
-- pada form transaksi.
INSERT INTO member (no_hp, nama, tgl_daftar) VALUES
  ('081234567890', 'Budi Santoso',  '2026-01-15'),
  ('081298765432', 'Siti Aminah',   '2026-02-20'),
  ('085611223344', 'Andi Wijaya',   '2026-03-05');

-- Promo contoh
-- (a) Diskon % produk tertentu -- hanya berlaku jam pagi 06:00-11:00
INSERT INTO promo (kode, nama, jenis, nilai_persen, tgl_mulai, tgl_selesai, jam_mulai, jam_selesai, cabang_ids, aktif) VALUES
  ('PROMO-KOPIMORNING', 'Diskon Kopi Pagi 10%', 'DISKON_PRODUK', 10, '2026-01-01', '2026-12-31', '06:00:00', '11:00:00', '', 1);

-- Detail produk yang mendapat diskon 10% (produk id 1 = Kopi Susu Gula Aren)
INSERT INTO promo_produk (promo_id, produk_id, persen)
SELECT id, 1, 10 FROM promo WHERE kode='PROMO-KOPIMORNING';

-- (b) Potongan nominal Rp15.000 dengan syarat minimal belanja Rp100.000.
-- Promotional ini TIDAK boleh digabung dengan voucher (BR-11).
INSERT INTO promo (kode, nama, jenis, nilai_nominal, min_belanja, tgl_mulai, tgl_selesai, cabang_ids, aktif) VALUES
  ('PROMO-HEMAT15K', 'Hemat Rp15.000 min belanja Rp100.000', 'POTONGAN_MIN_BELANJA', 15000, 100000, '2026-01-01', '2026-12-31', '', 1);

-- (c) Beli 2 gratis 1 per kategori (unit digabung dalam kategori)
INSERT INTO promo (kode, nama, jenis, tgl_mulai, tgl_selesai, cabang_ids, aktif) VALUES
  ('PROMO-B2G1KOPI', 'Beli 2 Gratis 1 Kopi', 'B2G1', '2026-01-01', '2026-12-31', '', 1),
  ('PROMO-B2G1MAKAN', 'Beli 2 Gratis 1 Makanan', 'B2G1', '2026-01-01', '2026-12-31', '', 1);

INSERT INTO promo_kategori (promo_id, kategori)
SELECT id, 'Kopi' FROM promo WHERE kode='PROMO-B2G1KOPI';
INSERT INTO promo_kategori (promo_id, kategori)
SELECT id, 'Makanan' FROM promo WHERE kode='PROMO-B2G1MAKAN';

-- (d) Voucher berkode berkuota: kode SEDUH10K, kuota 100 pemakaian.
-- Kuota bertambah saat transaksi disimpan, berkurang saat pembatalan.
INSERT INTO promo (kode, nama, jenis, nilai_nominal, tgl_mulai, tgl_selesai, cabang_ids, aktif) VALUES
  ('PROMO-VOUCHER10K', 'Voucher Diskon Rp10.000', 'VOUCHER', 10000, '2026-01-01', '2026-12-31', '', 1);

INSERT INTO voucher (kode, promo_id, kuota, terpakai)
SELECT 'SEDUH10K', id, 100, 0 FROM promo WHERE kode='PROMO-VOUCHER10K';
