package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql" // Import the MySQL driver
	"github.com/joho/godotenv"
)

// -----------------------------------------------------------------------------
// Models
// -----------------------------------------------------------------------------

type Upload struct {
	ID              int       `json:"id"`
	Filename        string    `json:"filename"`
	SourcePath      string    `json:"source_path"`
	DestinationPath string    `json:"destination_path"`
	UploadTime      time.Time `json:"upload_time"`
	Status          string    `json:"status"`
	ErrorMessage    string    `json:"error_message"`
}

func InitDB() (*sql.DB, error) {
	// Retrieve database credentials from environment variables
	// dbUser := os.Getenv("DB_USER")
	// dbPass := os.Getenv("DB_PASS")
	// dbHost := os.Getenv("DB_HOST")
	// dbName := os.Getenv("DB_NAME")

	// Construct the connection string
	dsn := "nora:root@root12@tcp(localhost:3306)/filedb?charset=utf8mb4&parseTime=True&loc=Local"

	// Open a connection to the database
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
		return nil, err
	}

	fmt.Println("Successfully connected to the database!")
	return db, nil
}

// insertUploadRecord inserts a new upload record into the database.
func insertUploadRecord(db *sql.DB, upload Upload) (int64, error) {
	query := `INSERT INTO uploads (filename, source_path, destination_path, status) VALUES (?, ?, ?, ?)`
	result, err := db.Exec(query, upload.Filename, upload.SourcePath, upload.DestinationPath, upload.Status)
	if err != nil {
		return 0, err
	}

	insertID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return insertID, nil
}

// updateUploadStatus updates the status and error message of an upload record in the database.
func updateUploadStatus(db *sql.DB, uploadID int, status, errorMessage string) error {
	query := `UPDATE uploads SET status = ?, error_message = ? WHERE id = ?`
	_, err := db.Exec(query, status, errorMessage, uploadID)
	return err
}

// getUploadByID retrieves an upload record from the database by ID.
func getUploadByID(db *sql.DB, uploadID int) (Upload, error) {
	var upload Upload
	query := `SELECT id, filename, source_path, destination_path, upload_time, status, error_message FROM uploads WHERE id = ?`
	err := db.QueryRow(query, uploadID).Scan(&upload.ID, &upload.Filename, &upload.SourcePath, &upload.DestinationPath, &upload.UploadTime, &upload.Status, &upload.ErrorMessage)
	return upload, err
}

func moveFile(sourcePath, destinationPath string) error {
	err := os.Rename(sourcePath, destinationPath)
	if err != nil {
		return err
	}
	return nil
}

// UploadFileHandler handles the file upload and database interaction.
func UploadFileHandler(db *sql.DB, uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Check if the request method is POST
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 2. Parse the multipart form data.
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit (adjust as needed)
		if err != nil {
			http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		// 3. Get the file from the form data.
		file, header, err := r.FormFile("file") // "file" is the name of the form field
		if err != nil {
			http.Error(w, "Failed to get file: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// 4. Define source and destination paths.
		filename := header.Filename
		sourcePath := filepath.Join(os.TempDir(), filename)   // Temporary storage
		destinationPath := filepath.Join(uploadDir, filename) // Final destination

		// 5. Save the uploaded file to the source path.
		outFile, err := os.Create(sourcePath)
		if err != nil {
			http.Error(w, "Failed to create temporary file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, file)
		if err != nil {
			http.Error(w, "Failed to save file to temporary location: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 6. Create a database record.
		upload := Upload{
			Filename:        filename,
			SourcePath:      sourcePath,
			DestinationPath: destinationPath,
			Status:          "pending",
		}

		// Insert the upload record into the database
		insertID, err := insertUploadRecord(db, upload)
		if err != nil {
			log.Printf("Error inserting upload record: %v", err)
			http.Error(w, "Failed to create database record: "+err.Error(), http.StatusInternalServerError)

			// Optionally, remove the uploaded file if DB insertion fails
			err := os.Remove(sourcePath)
			if err != nil {
				log.Printf("Error removing temporary file after DB insertion failure: %v", err)
			}
			return
		}
		upload.ID = int(insertID) // Set the ID from the database

		// 7. Move the file from source to destination (in a separate goroutine for async processing).
		go func(upload Upload) {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("pkg: %v", r)
					}
					log.Printf("Panic during file move: %v", err)
					updateUploadStatus(db, upload.ID, "failed", fmt.Sprintf("Panic: %v", err))
				}
			}()

			err := moveFile(upload.SourcePath, upload.DestinationPath)
			if err != nil {
				log.Printf("Error moving file: %v", err)
				// Update the database record with the error
				updateUploadStatus(db, upload.ID, "failed", err.Error())
			} else {
				// Update the database record with success status
				updateUploadStatus(db, upload.ID, "success", "")
			}
		}(upload)

		// 8. Respond to the client.
		w.WriteHeader(http.StatusCreated)
		responseMessage := fmt.Sprintf(`{"message": "File uploaded successfully. Processing in the background.", "uploadID": %d}`, upload.ID)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(responseMessage)))
		w.Write([]byte(responseMessage))
	}
}

// GetUploadStatusHandler retrieves the status of an upload by ID.
func GetUploadStatusHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		uploadIDStr := r.URL.Query().Get("uploadID") // Get uploadID from query parameter
		if uploadIDStr == "" {
			http.Error(w, "uploadID is required", http.StatusBadRequest)
			return
		}

		uploadID, err := strconv.Atoi(uploadIDStr)
		if err != nil {
			http.Error(w, "Invalid uploadID", http.StatusBadRequest)
			return
		}

		upload, err := getUploadByID(db, uploadID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Upload not found", http.StatusNotFound)
			} else {
				log.Printf("Error getting upload by ID: %v", err)
				http.Error(w, "Failed to retrieve upload", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(upload); err != nil {
			log.Printf("Error encoding JSON: %v", err)
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	}
}

// ListFilesHandler lists the files in the upload directory.
func ListFilesHandler(uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		files := []string{} // Initialize an empty slice to store filenames

		err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err // Propagate the error if any
			}

			if !info.IsDir() {
				// Remove the base path (uploadDir) to get relative filename
				relativePath, err := filepath.Rel(uploadDir, path)
				if err != nil {
					return err // Handle relative path error
				}
				files = append(files, relativePath) // Append the relative path
			}
			return nil
		})

		if err != nil {
			log.Printf("Error walking the upload directory: %v", err)
			http.Error(w, "Failed to list files", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(files); err != nil {
			log.Printf("Error encoding JSON: %v", err)
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	}
}

func main() {
	// Load environment variables from .env (if present)
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file.  Using environment variables directly.")
	}

	// Initialize database connection
	db, err := InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Define the upload directory (read from environment or default)
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads" // Default upload directory
	}

	// Create the upload directory if it doesn't exist
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.MkdirAll(uploadDir, 0755); err != nil { // Create recursively
			log.Fatalf("Failed to create upload directory: %v", err)
		}
	}

	// Register the upload handler
	http.HandleFunc("/upload", UploadFileHandler(db, uploadDir)) // Pass DB and uploadDir
	http.HandleFunc("/upload_status", GetUploadStatusHandler(db))
	http.HandleFunc("/list_files", ListFilesHandler(uploadDir)) // List files

	// Get the port from the environment, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start the server
	fmt.Printf("Server listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
