# Script Download Modul UT

Script untuk mendownload modul dari website dan menggabungkannya menjadi satu file PDF.

## Fitur

- Download semua file gambar secara otomatis
- Menggabungkan semua halaman menjadi 1 file PDF
- Konfigurasi via file `.env`
- Rate limiting untuk menghindari block dari server

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

## Penggunaan

```bash
go run .
```

Script akan:

1. Download semua halaman dari M1 sampai M9
2. Menyimpan gambar sementara di folder `temp_images/`
3. Menggabungkan semua gambar menjadi PDF
4. Menghapus folder temporary

## Output

File PDF akan disimpan dengan nama sesuai `OUTPUT_NAME` di `.env`.

## Catatan

- Pastikan session (PHPSESSID) masih aktif
- Jika download gagal terus, kemungkinan IP di-rate limit. Tunggu beberapa menit.
- Delay antar request: 1 detik, antar modul: 3 detik

## License

MIT
