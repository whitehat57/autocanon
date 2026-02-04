# Alat Pengujian Benchmark Autocanon

## Gambaran Umum

Autocanon adalah alat pengujian beban dan benchmark HTTP berkinerja tinggi yang ditulis dalam bahasa Go. Alat ini mensimulasikan permintaan konkuren ke URL target untuk mengukur metrik kinerja seperti laten, throughput, dan tingkat respons. Alat ini dirancang untuk menguji stres server web dan API dengan mengirim jumlah permintaan konkuren yang dapat dikonfigurasi.

## Tujuan dan Fungsionalitas

Tujuan utama alat ini adalah untuk:
- Mengukur kinerja server web/API di bawah beban
- Menghasilkan pola lalu lintas yang realistis untuk mensimulasikan penggunaan dunia nyata
- Mengumpulkan dan menganalisis metrik kinerja termasuk persentil laten, tingkat permintaan, dan throughput
- Mengidentifikasi bottleneck dan masalah kinerja dalam aplikasi web

## Arsitektur dan Komponen

### Komponen Utama

1. **Fungsi Utama (`main()`)**: Titik masuk yang menangani argumen baris perintah, menginisialisasi proses benchmarking, dan mengoordinasikan pekerja.

2. **Pool Pekerja (`runWorkers()`)**: Membuat dan mengelola goroutine yang melakukan permintaan HTTP sebenarnya.

3. **Sistem Pengumpulan Metrik**: Menggunakan HDR Histogram untuk mengumpulkan dan menganalisis data kinerja.

4. **Sistem Tampilan Hasil**: Memformat dan menampilkan metrik yang dikumpulkan dalam tabel yang dapat dibaca manusia.

### Struktur Data

#### Struktur Respons (`resp`)
```go
type resp struct {
    status  int   // Kode status HTTP dari respons
    latency int64 // Waktu yang dibutuhkan untuk permintaan dalam milidetik
    size    int   // Ukuran respons dalam byte
}
```

Struktur ini menangkap informasi penting tentang setiap permintaan HTTP untuk analisis selanjutnya.

## Alur Kerja

### 1. Fase Inisialisasi
- Parsing flag baris perintah untuk mengkonfigurasi benchmark
- Validasi URL target
- Inisialisasi channel untuk komunikasi antara pekerja dan thread utama
- Siapkan histogram HDR untuk pengumpulan metrik
- Buat konteks dengan pembatalan untuk shutdown yang mulus

### 2. Fase Peluncuran Pekerja
- Hitung jumlah total pekerja konkuren berdasarkan klien × faktor pipelining
- Luncurkan goroutine pekerja menggunakan fungsi `runWorkers`
- Setiap pekerja mempertahankan klien HTTP sendiri dengan pooling koneksi

### 3. Fase Eksekusi Benchmark
- Mulai timer untuk durasi yang ditentukan
- Luncurkan spinner untuk memberikan umpan balik visual
- Terus pantau channel untuk respons dan kesalahan
- Rekam metrik dalam histogram HDR setiap detik
- Tangani sinyal timeout dan pembatalan

### 4. Fase Pemrosesan Hasil
- Hentikan semua pekerja secara mulus
- Tunggu sebentar untuk menguras respons tersisa dari channel
- Hitung dan tampilkan metrik statistik
- Cetak statistik ringkasan termasuk jumlah kesalahan

## Detail Implementasi Teknis

### Model Konkurensi
Alat ini menggunakan goroutine dan channel Go untuk pemrosesan konkuren:
- Banyak goroutine pekerja mengeksekusi permintaan HTTP secara paralel
- Channel (`respChan`, `errChan`) memfasilitasi komunikasi antara pekerja dan thread utama
- Konteks dengan pembatalan memungkinkan shutdown mulus semua pekerja

### Konfigurasi Klien HTTP
- Menggunakan `fasthttp.Client` untuk operasi HTTP berkinerja tinggi
- Mengkonfigurasi pooling koneksi dengan `MaxConnsPerHost`
- Menetapkan timeout baca/tulis untuk mencegah permintaan yang menggantung
- Menggunakan metode POST dengan payload tetap ("hello, world!")

### Pengumpulan Metrik
Alat ini menggunakan HDR Histogram untuk pengukuran statistik yang akurat:
- **Histogram Laten**: Melacak waktu respons dengan presisi tinggi
- **Histogram Permintaan**: Mengukur permintaan per detik
- **Histogram Throughput**: Mencatat byte yang ditransfer per detik
- Menggunakan perhitungan persentil (2.5%, 50%, 97.5%, 99%) untuk analisis menyeluruh

### Manajemen Memori
- Menggunakan `fasthttp.AcquireRequest()` dan `fasthttp.AcquireResponse()` untuk meminimalkan alokasi
- Melepaskan sumber daya dengan benar dengan `defer fasthttp.ReleaseRequest()` dan `defer fasthttp.ReleaseResponse()`
- Mengatur ulang badan respons untuk memungkinkan penggunaan ulang koneksi

## Opsi Baris Perintah

| Flag | Bawaan | Deskripsi |
|------|--------|-----------|
| `-url` | (wajib) | URL target untuk benchmarking |
| `-c` | 10 | Jumlah koneksi konkuren |
| `-p` | 1 | Faktor pipelining (permintaan per koneksi) |
| `-d` | 10 | Durasi benchmark dalam detik |
| `-t` | 10 | Timeout koneksi dalam detik |
| `-debug` | false | Aktifkan output debug |

## Fitur Utama

### Kinerja Tinggi
- Memanfaatkan model konkurensi Go yang efisien
- Menggunakan pustaka fasthttp untuk operasi HTTP yang dioptimalkan
- Menerapkan pooling koneksi untuk mengurangi overhead

### Metrik Lengkap
- Persentil laten (2.5%, 50%, 97.5%, 99%)
- Statistik tingkat permintaan
- Pengukuran throughput dalam format yang dapat dibaca manusia
- Pelacakan kesalahan dan timeout

### Umpan Balik Visual
- Spinner animasi selama eksekusi benchmark
- Tabel output dengan pewarnaan
- Format angka yang dapat dibaca manusia untuk keterbacaan

### Shutdown yang Mulus
- Pembatalan berbasis konteks untuk terminasi pekerja yang bersih
- Penundaan singkat untuk menguras respons tersisa dari channel
- Pembersihan sumber daya yang tepat

## Pustaka yang Digunakan

- **fasthttp**: Pustaka klien/server HTTP berkinerja tinggi
- **hdrhistogram-go**: High Dynamic Range Histogram untuk pengukuran statistik yang akurat
- **spinner**: Spinner terminal untuk umpan balik visual
- **tablewriter**: Output tabel terformat
- **chalk**: Utilitas warna/gaya terminal
- **humanize**: Format angka dan unit yang dapat dibaca manusia

## Penanganan Kesalahan

Alat ini menerapkan penanganan kesalahan yang kuat:
- Memvalidasi format URL saat startup
- Menangkap dan mengategorikan berbagai jenis kesalahan
- Membedakan antara kesalahan umum dan timeout
- Memberikan output debug saat diaktifkan

## Pertimbangan Kinerja

- Manajemen goroutine yang efisien untuk mencegah kehabisan sumber daya
- Buffering channel untuk menangani lonjakan respons
- Penanganan permintaan/respons yang efisien dalam memori
- Penggunaan ulang koneksi untuk meminimalkan overhead handshake

## Catatan Keamanan

⚠️ **Peringatan**: Alat ini dirancang untuk pengujian kinerja yang sah. Penyalahgunaan untuk serangan penolakan layanan terhadap sistem tanpa otorisasi adalah ilegal dan tidak etis. Hanya gunakan pada sistem yang Anda miliki atau memiliki izin eksplisit untuk menguji.
