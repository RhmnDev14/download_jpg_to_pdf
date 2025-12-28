# Script Download Modul

Script untuk mendownload modul dari website dan menggabungkannya menjadi satu file PDF.

## Fitur

- Download semua file gambar secara otomatis
- Menggabungkan semua halaman menjadi 1 file PDF
- Konfigurasi via file `.env`
- Rate limiting untuk menghindari block dari server
- Kirim PDF ke WhatsApp via WAHA API

## Instalasi

```bash
# Clone repository
git clone <repo-url>
cd script-download-modul

# Install dependencies
go mod download
```

## Konfigurasi

1. Copy file `.env.example` menjadi `.env`:

   ```bash
   cp .env.example .env
   ```

2. Edit `.env` dengan nilai yang sesuai:
   ```env
   SUBFOLDER=KODE_MODUL/
   PHPSESSID=your_session_id_here
   OUTPUT_NAME=nama_output_pdf
   ```

### Konfigurasi WhatsApp (Opsional)

Untuk mengirim PDF otomatis ke WhatsApp setelah selesai download, tambahkan konfigurasi WAHA:

```env
# WAHA WhatsApp API Configuration
WAHA_API_URL=https://your-waha-server.com
WAHA_API_KEY=your_api_key
WAHA_SESSION=default
WAHA_RECIPIENT=628123456789
```

| Parameter        | Deskripsi                                  |
| ---------------- | ------------------------------------------ |
| `WAHA_API_URL`   | URL server WAHA API                        |
| `WAHA_API_KEY`   | API key untuk autentikasi                  |
| `WAHA_SESSION`   | Nama session WhatsApp (default: `default`) |
| `WAHA_RECIPIENT` | Nomor WhatsApp tujuan (format: `628xxx`)   |

> **Note:** Jika konfigurasi WAHA tidak diisi, fitur pengiriman WhatsApp akan dilewati.

## Penggunaan

```bash
go run .
```

Script akan:

1. Download semua halaman dari M1 sampai M9
2. Menyimpan gambar sementara di folder `temp_images/`
3. Menggabungkan semua gambar menjadi PDF
4. Menghapus folder temporary
5. Mengirim PDF ke WhatsApp (jika WAHA dikonfigurasi)

## Output

File PDF akan disimpan dengan nama sesuai `OUTPUT_NAME` di `.env`.

## Catatan

- Pastikan session (PHPSESSID) masih aktif
- Jika download gagal terus, kemungkinan IP di-rate limit. Tunggu beberapa menit.
- Delay antar request: 1 detik, antar modul: 3 detik
- Pengiriman file ke WhatsApp menggunakan multipart upload streaming

## License

MIT
