# File Upload API 📁

A simple and efficient Go-based API for handling file uploads with MySQL database integration.

## Features ✨

- Asynchronous file upload processing 📤
- Database tracking of upload status 📑
- Secure file storage 🔒
- Easy-to-use REST endpoints 🛠️
- File listing capability 📋

## Getting Started 🚀

### Prerequisites

- Go 1.x
- MySQL Database
- godotenv package
- MySQL driver for Go

### Installation 💻

1. Clone the repository:
```bash
git clone https://github.com/Amul-Thantharate/file-uplaod-api.git
cd file-uplaod-api
```

2. Set up your environment variables in `.env`:
```bash
DB_USER=your_db_user
DB_PASS=your_db_password
DB_HOST=localhost
DB_NAME=filedb
UPLOAD_DIR=uploads
PORT=8080
```

3. Create the MySQL database and table:
```sql
CREATE DATABASE filedb;
USE filedb;
CREATE TABLE uploads (
    id INT AUTO_INCREMENT PRIMARY KEY,
    filename VARCHAR(255),
    source_path VARCHAR(255),
    destination_path VARCHAR(255),
    upload_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50),
    error_message TEXT
);
```

4. Run the application:
```bash
go run main.go
```

## API Endpoints 🔌

### Upload a File 📤

```bash
curl -X POST \
  http://localhost:8080/upload \
  -H "Content-Type: multipart/form-data" \
  -F "file=@/path/to/your/file.txt"
```

**Response** ✅
```json
{
    "message": "File uploaded successfully. Processing in the background.",
    "uploadID": 123
}
```

### Check Upload Status 📊

```bash
curl -X GET http://localhost:8080/upload_status?id=123
```

**Response** ✅
```json
{
    "id": 123,
    "filename": "file.txt",
    "source_path": "/tmp/file.txt",
    "destination_path": "/uploads/file.txt",
    "upload_time": "2025-02-27T08:04:35Z",
    "status": "success",
    "error_message": ""
}
```

### List Files 📋

```bash
curl -X GET http://localhost:8080/list_files
```

**Response** ✅
```json
{
    "files": [
        "file1.txt",
        "file2.pdf",
        "image.jpg"
    ]
}
```

## Error Handling ⚠️

The API returns appropriate HTTP status codes and error messages:

- `400` - Bad Request (Failed to parse form/get file)
- `405` - Method Not Allowed
- `500` - Internal Server Error (Database/File system errors)

Upload statuses:
- `pending` - File upload initiated
- `success` - File successfully processed
- `failed` - Processing failed (with error message)

## Security Considerations 🔐

- File size limit of 10MB enforced
- Asynchronous file processing
- Secure file storage implementation
- Database tracking of all uploads
- Error handling and recovery
- Panic recovery in background processing

## Contributing 🤝

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License 📝

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support 💬

For support, please open an issue in the GitHub repository or contact the maintainers.

---
Made with ❤️ by [Amul-Thantharate](https://github.com/Amul-Thantharate)
