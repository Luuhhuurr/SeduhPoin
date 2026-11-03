# SeduhPoin — POS + Loyalty Program "Kedai Kopi Seduh"

Aplikasi web Point of Sale (POS) dengan program loyalitas untuk **Kedai Kopi Seduh**
(3 cabang). Dibuat sebagai tugas akhir menggunakan **Go murni** (tanpa framework web)
dan **MySQL** (XAMPP) tanpa ORM.

| Aspek | Teknologi |
|---|---|
| Bahasa | Go (paket standar + 2 dependensi) |
| Web server | `net/http` + `html/template` (bukan Gin/Echo/Fiber) |
| Database | MySQL (XAMPP/phpMyAdmin) via `database/sql` (bukan ORM) |
| Frontend | HTML + CSS murni (bukan React/Vue, tanpa build tool) |
| Dependensi | `github.com/go-sql-driver/mysql`, `github.com/joho/godotenv` |

---

## 1. Setup dari Nol

### Langkah 1 — Jalankan MySQL XAMPP

1. Buka **XAMPP Control Panel** → klik **Start** pada modul **MySQL**.
2. Pastikan statusnya *Running* (port 3306).

Alternatif tanpa XAMPP Control Panel:

```bash
C:/xampp/mysql/bin/mysqld.exe --defaults-file=C:/xampp/mysql/bin/my.ini --console
```

### Langkah 2 — Buat database dan import skema

**Cara A — Import file `schema.sql` (paling cepat, disarankan)**

1. Buka <http://localhost/phpmyadmin>.
2. Klik tab **Import** di menu atas.
3. Klik **Choose File**, pilih `schema.sql` dari folder proyek, lalu klik **Go**.

Tidak perlu membuat database secara manual: `schema.sql` sudah memuat
`CREATE DATABASE`, `USE`, seluruh `CREATE TABLE` (InnoDB), index, foreign key,
serta data awal (seed).

**Cara B — Buat database dulu lewat tab SQL**

Kode pendek ini (tempel di tab **SQL**, lalu klik **Go**):

```sql
-- 1. Buat database
CREATE DATABASE IF NOT EXISTS `seduhpoin`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_general_ci;

-- 2. Pilih database
USE `seduhpoin`;

-- 3. (Opsional) User khusus aplikasi, lebih aman daripada memakai root
CREATE USER IF NOT EXISTS 'seduhpoin'@'localhost' IDENTIFIED BY 'seduhpoin123';
GRANT ALL PRIVILEGES ON `seduhpoin`.* TO 'seduhpoin'@'localhost';
FLUSH PRIVILEGES;
```

Bila memakai user khusus, sesuaikan `.env`:

```env
DB_DSN=seduhpoin:seduhpoin123@tcp(127.0.0.1:3306)/seduhpoin?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta
```

Setelah database dibuat, **import `schema.sql`** (Cara A) untuk membuat tabel dan
mengisi data awal.

Verifikasi hasil import:

```sql
USE `seduhpoin`;
SHOW TABLES;                    -- 12 tabel
SELECT COUNT(*) FROM cabang;    -- 3
SELECT COUNT(*) FROM produk;    -- 12
SELECT COUNT(*) FROM member;    -- 3
SELECT COUNT(*) FROM promo;     -- 5
SELECT COUNT(*) FROM voucher;   -- 1
```

> **Penting:** `schema.sql` diawali `DROP TABLE IF EXISTS`, sehingga menjalankannya
> ulang akan **menghapus dan membuat ulang** seluruh tabel beserta datanya.
> Lakukan hanya saat setup pertama atau ketika ingin mengulang dari nol.

### Langkah 3 — Isi file `.env`

Salin `.env.example` menjadi `.env` lalu sesuaikan:

```env
DB_DSN=root:@tcp(127.0.0.1:3306)/seduhpoin?parseTime=true&charset=utf8mb4&loc=Asia%2FJakarta
PORT=8080
```

> **Penting:** nilai `loc` harus ditulis `Asia%2FJakarta` (persen di-encode).
> Driver MySQL memisahkan DSN pada karakter `/` terakhir, sehingga `loc=Asia/Jakarta`
> akan membuat DSN gagal parsing (`invalid DSN: did you forget to escape a param value?`).

### Langkah 4 — Ambil dependensi & jalankan

```bash
go mod tidy
go run .
```

### Langkah 5 — Buka di browser

<http://localhost:8080>

> **Tidak ada login.** Seluruh menu dapat diakses langsung; aplikasi ini memakai
> satu peran (kasir/admin) sesuai spesifikasi tugas akhir. Tidak ada mekanisme
> autentikasi maupun sesi.

### Perintah yang sering dipakai

```bash
go build ./...        # cek kompilasi
go test ./...         # jalankan unit test
go test ./internal/calc -v   # unit test aturan bisnis (detail)
gofmt -l .            # cek format kode
```

---

## 2. Daftar Menu

| Menu | URL | Keterangan |
|---|---|---|
| Dashboard | `/` | Ringkasan transaksi, promo aktif, member |
| Transaksi Baru | `/transaksi` | Form transaksi: pilih cabang, item, member, voucher, tukar poin |
| Riwayat Transaksi | `/riwayat` | Daftar transaksi + filter tanggal/cabang, detail, pembatalan |
| Produk | `/produk` | CRUD produk (soft delete) |
| Member | `/member` | CRUD member, pencarian berdasarkan nomor HP |
| Promo & Voucher | `/promo` | CRUD 4 jenis promo + pengelolaan kuota voucher |
| Cabang | `/cabang` | CRUD cabang (soft delete) |
| Poin Member | `/poin` | Saldo poin aktif, tingkat member, lot poin, mutasi poin |
| Laporan Biaya Promo | `/laporan/promo` | FR-18 |
| Laporan Poin Berlaku | `/laporan/poin` | FR-19 |
| Laporan Member per Tingkat | `/laporan/tingkat` | FR-20 |

Data awal (seed): nomor HP member `081234567890`, `081298765432`, `085611223344`;
kode voucher `SEDUH10K` (kuota 100).

---

## 3. Struktur Folder

```
SeduhPoin/
├── main.go                     koneksi DB, router, server
├── go.mod / go.sum
├── .env / .env.example
├── schema.sql                  DDL + index + seed data
├── README.md
├── internal/
│   ├── calc/                   ATURAN BISNIS (pure, unit-testable)
│   │   ├── calc.go             seluruh perhitungan & konstanta
│   │   └── calc_test.go        unit test wajib
│   ├── models/                 struktur domain
│   ├── format/                 pemformat angka, rupiah, tanggal (WIB)
│   └── store/                  akses database (SQL mentah, sql.Tx)
│       ├── db.go, cabang.go, produk.go, member.go,
│       ├── promo.go, poin.go, transaksi.go, laporan.go
├── handlers/                   HTTP handler + template
│   ├── app.go, helpers.go, render.go
│   ├── dashboard.go, cabang.go, produk.go, member.go, promo.go
│   ├── transaksi.go, riwayat.go, poin.go, laporan.go
├── templates/                  layout, header, sidebar, footer + halaman
└── static/style.css            seluruh gaya tampilan
```

Pola yang dipakai:

* `internal/calc` **tidak** menyentuh database sehingga seluruh aturan bisnis dapat
  diuji dengan `go test`. Fungsi `calc.Hitung()` dipakai oleh handler "Hitung"
  maupun saat menyimpan transaksi, sehingga perhitungan **tidak pernah diduplikasi**.
* `internal/store` menangani seluruh SQL; penulisan atomic memakai `sql.Tx`.
* `handlers` hanya mengatur HTTP, validasi input, dan pemanggilan template.

---

## 4. Aturan Bisnis (ringkasan)

Semua perhitungan berada di `internal/calc/calc.go`.

**Poin**

* 1 poin untuk setiap kelipatan **Rp10.000** dari total **setelah promo** (sisa di bawah
  Rp10.000 diabaikan).
* Dasar poin = total setelah promo; poin diperoleh = `floor(dasar x pengali)`.
* Pengali: **Silver x1.0**, **Gold x1.5**.
* Tingkat **tidak disimpan** di tabel member — dihitung on-the-fly dari total belanja
  3 bulan terakhir atas transaksi **SELESAI**. Ambang Gold: **Rp1.500.000**.
* Pengali yang dipakai sebuah transaksi adalah tingkat **pada saat transaksi dimulai**
  (sebelum transaksi itu ikut dihitung), dan disimpan pada
  `transaksi.tingkat_saat_transaksi` + `pengali_saat_transaksi`.
* Masa berlaku poin **6 bulan** (sampai akhir hari tanggal yang sama 6 bulan kemudian).
* Nilai tukar **1 poin = Rp1.000**.

**Tukar poin**

* Maksimal **50% dari total setelah promo**:
  `batas = min(saldo poin aktif, floor(50% x total setelah promo / 1.000))`.
* Penukaran memakai **FIFO**: lot dengan tanggal kedaluarsa terdekat dipakai lebih dulu;
  bila sama, yang diperoleh lebih dulu didahulukan.
* Hanya **member terdaftar** yang memperoleh dan menukar poin. Non-member tetap
  bisa memakai promo.

**Promo (4 jenis)**

| Jenis | Aturan |
|---|---|
| Diskon produk | Persentase diskon untuk produk tertentu |
| Potongan minimal belanja | Potongan nominal bila belanja mencapai minimum |
| Beli 2 gratis 1 | Setiap 3 unit dalam satu kategori → 1 unit termurah gratis (unit digabung per kategori) |
| Voucher | Kode unik, berkuota, maksimal 1 voucher per transaksi |

* Promo produk (diskon % dan B2G1) **boleh digabung**.
* Promo **potongan minimal belanja tidak boleh digabung dengan voucher**. Bila kode
  voucher diisi, potongan minimal belanja **tidak** diterapkan (BR-11).
* Promo memiliki masa berlaku, restriction cabang (kosong = semua cabang), dan
  rentang jam (kosong = sepanjang hari).
* Promo yang **sudah dipakai** hanya boleh **dinonaktifkan**, tidak dapat dihapus.

**Urutan perhitungan transaksi (WAJIB)**

1. `subtotal` = Σ (harga x jumlah)
2. Promo produk: **B2G1** dulu, lalu **diskon % produk**
3. **Salah satu**: voucher **ATAU** potongan minimal belanja (dihitung dari belanja
   setelah promo produk)
4. `totalSetelahPromo` (minimal Rp0) = dasar poin **dan** dasar batas tukar poin
5. Tukar poin (FIFO) → `totalBayar = totalSetelahPromo - poin x Rp1.000`
6. Setelah disimpan: `poinDiperoleh = floor(totalSetelahPromo / 10.000) x pengali`,
   lalu dibuat **lot poin baru** (kedaluarsa +6 bulan, status AKTIF)

**Pembatalan (BR-14)**

* Transaksi berstatus **BATAL** (pembatalan utuh, bukan parsial); data **tidak dihapus**.
* Poin yang diperoleh ditarik (lot → `DITARIK`, mutasi `TARIK`).
* Poin yang ditukar dikembalikan ke lot asalnya (mutasi `KEMBALI`).
* Kuota voucher dikembalikan (`terpakai - 1`).
* Biaya promo keluar dari laporan karena laporan hanya menghitung transaksi `SELESAI`.
* Pembatalan **ditolak** bila poin yang diperoleh dari transaksi tersebut sudah
  dipakai pada transaksi lain.

**Contoh angka (sudah menjadi unit test)**

```
Subtotal                Rp 145.000
- B2G1                  Rp  20.000
- Diskon produk          Rp   6.000
- Potongan min belanja   Rp  15.000
= Total setelah promo    Rp 104.000
Tukar 40 poin          - Rp  40.000
= Total bayar            Rp  64.000
Poin diperoleh = floor(104.000 / 10.000) x 1.5 (Gold) = 15 poin
```

---

## 5. Unit Test

```bash
go test ./internal/calc -v      # aturan bisnis (perhitungan transaksi & poin)
go test ./internal/models -v    # validitas promo (masa berlaku, jam, cabang, status)
```

Test yang tersedia:

| Test | Isi |
|---|---|
| `TestContohAngkaSubbab22` | Contoh angka wajib di atas (Rp145.000 → Rp104.000 → Rp64.000, 15 poin) |
| `TestContohAngkaTanpaMember` | Non-member memakai promo tanpa memperoleh/menukar poin |
| `TestVoucherTidakDigabungDenganPotonganMinBelanja` | BR-11: voucher dan potongan min belanja tidak digabung |
| `TestPoinMelebihiBatas` | Penukaran di atas batas 50% ditolak |
| `TestFIFODuaLot` | FIFO 2 lot + lot kedaluwarsa dilewati |
| `TestFIFOSamaKedaluarsa` | Tie-break FIFO: yang diperoleh lebih dulu |
| `TestTingkatGoldBoundary` | Rp1.499.999 = Silver, Rp1.500.000 = Gold |
| `TestPoinDiperoleh` | Dasar poin dan pengali tingkat |
| `TestBatasTukarPoin` | Batas tukar 50% |
| `TestTanggalKedaluarsa` | Masa berlaku 6 bulan (termasuk 31 Agu → 28 Feb) |
| `TestB2G1GabungKategori` | Unit digabung dalam satu kategori |
| `TestBerlakuPadaMasaBerlaku` | Promo hanya berlaku antara `tgl_mulai` dan `tgl_selesai` |
| `TestBerlakuPadaJam` | Promo berkode jam (kosong = sepanjang hari) |
| `TestBerlakuPadaCabang` | Promo berkode cabang (kosong = semua cabang) |
| `TestBerlakuPadaStatusAktif` | Promo nonaktif tidak dapat dipakai |

---

## 6. Keputusan Desain

1. **Tanpa login/sesi** — sesuai spesifikasi: satu peran kasir/admin.
2. **Tingkat member tidak disimpan** — dihitung on-the-fly dari transaksi 3 bulan
   terakhir, sehingga selalu akurat tanpa perlu job pemeliharaan data.
3. **`cabang_ids` disimpan sebagai CSV** (`VARCHAR`) pada tabel `promo`; kosong berarti
   berlaku untuk semua cabang. Cukup untuk 3 cabang dan menghindari tabel pivot.
4. **Jam promo disimpan sebagai string `"HH:MM:SS"`** pada kolom `TIME`, agar tidak
   bermasalah dengan parsing driver MySQL.
5. **Soft delete** untuk master data (kolom `deleted_at`); nama produk, harga, dan nama
   member **disalin** ke tabel transaksi agar transaksi lama tetap menampilkan data lama.
6. **Hapus master data tidak menghapus data historis** — seluruh transaksi lama tetap
   utuh dan terbaca di riwayat maupun laporan.
7. **Pemotongan promo produk** — satu produk yang terkena beberapa promo diskon % memakai
   **persentase terbesar**, dan potongan diatribusikan ke promo tersebut agar laporan
   biaya promo (FR-18) akurat.
8. **Biaya tukar poin pada laporan biaya promo** ditampilkan pada **kolom terpisah**
   dari total potongan promo, dan tidak dihitung dua kali bila satu transaksi memakai
   beberapa promo.
9. **`voucher_kode` disimpan pada tabel `transaksi`** agar kuota voucher dapat dikembalikan
   dengan tepat saat pembatalan.
10. **Kuota voucher dijaga dengan satu pernyataan**:
    `UPDATE voucher SET terpakai = terpakai + 1 WHERE kode = ? AND terpakai < kuota`,
    lalu jumlah baris yang terpengaruh diperiksa — sehingga kuota tidak mungkin terpakai
    melebihi kuota.
11. **Penguncian baris poin** memakai `SELECT ... FOR UPDATE` pada tabel `poin_lot`
    sehingga satu lot tidak dipakai dua kali oleh transaksi bersamaan.
12. **Tanpa `log.Fatal`** — kegagalan database tidak mematikan server; pengguna melihat
    halaman pesan yang jelas (503), server tetap berjalan.
13. **Zona waktu Asia/Jakarta** dijamin dengan `import _ "time/tzdata"` (Windows sering
    tidak menyediakan basis data zona waktu sistem) dan `loc=Asia%2FJakarta` pada DSN.
14. **Template diparse per-request** (tanpa cache) supaya perubahan tampilan langsung
    terlihat tanpa perlu restart server.
15. **JavaScript diminimalkan** — hanya satu fungsi untuk menambah baris item dan
    konfirmasi pembatalan. Perhitungan tidak pernah dilakukan di sisi browser: tombol
    "Hitung" mengirim POST ke server lalu server menampilkan kembali rinciannya.
16. **Sidebar dapat dibuka/tutup di layar kecil** memakai checkbox CSS murni
    (tanpa JavaScript), sehingga menu tetap bisa diakses di ponsel.
17. **Berkas statis memakai `Cache-Control: no-cache`** agar perubahan tampilan
    langsung terlihat tanpa perlu memaksa refresh keras di browser.
18. **Ikon memakai SVG inline** pada sidebar dan header, bukan font ikon pihak
    ketiga, sehingga tidak ada permintaan jaringan tambahan dan tetap sesuai
    larangan penggunaan framework frontend.

---

## 7. Catatan Teknis

* Semua nilai uang dan poin bertipe `int64` (tanpa `float` untuk nominal).
* **Tampilan**: satu berkas `static/style.css` (tanpa framework CSS) berisi
  design system sederhana: palet espresso + krem, sidebar bertingkat, header
  lengket, kartu statistik, tabel dengan header lengket, dan aturan responsif
  (sidebar menjadi drawer di layar < 900px).
* Semua waktu disimpan dan ditampilkan dalam zona **WIB (Asia/Jakarta)**.
* Penulisan transaksi dan pembatalan masing-masing berjalan dalam **satu `sql.Tx`**:
  transaksi, item, rincian promo, penggunaan voucher, mutasi tukar poin, dan pembuatan
  lot poin baru disimpan secara atomik.
* Waktu server (`created_at`, `tanggal_transaksi`, `batal_at`) menggunakan waktu
  aplikasi di zona WIB.

---

## 8. Batasan yang Ditegakkan

* Tidak memakai framework web (Gin/Echo/Fiber/Beego).
* Tidak memakai ORM (GORM atau lainnya); seluruh akses database memakai `database/sql`
  dengan SQL mentah.
* Tidak memakai React/Vue/Angular atau build tool pada frontend.
* Tidak ada autentikasi/login.
* Seluruh perhitungan aturan bisnis berada di `internal/calc` dan dapat diuji dengan
  `go test`.
