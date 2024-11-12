package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"github.com/rs/cors"
)

var db *sql.DB
var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

func main() {
	dbUser := os.Getenv("MYSQL_USER")
	dbPassword := os.Getenv("MYSQL_PASSWORD")
	dbName := os.Getenv("MYSQL_DATABASE")
	dbHost := "db"
	dbPort := "3306"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local", dbUser, dbPassword, dbHost, dbPort, dbName)

	var err error
	for i := 0; i < 5; i++ { // Intentar 5 veces
		var gormDB *gorm.DB
		gormDB, err = gorm.Open("mysql", dsn)
		if err == nil {
			db = gormDB.DB() // Obtener la conexión *sql.DB
			break // Salir del bucle si la conexión es exitosa
		}
		log.Println("Error al conectar a la base de datos, reintentando en 2 segundos...")
		time.Sleep(2 * time.Second) // Esperar 2 segundos antes de reintentar
	}

	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/api/login", Login).Methods("POST")
	router.HandleFunc("/api/register", Register).Methods("POST")
	router.HandleFunc("/api/dashboard", Dashboard).Methods("GET")
	router.HandleFunc("/api/users", GetUsers).Methods("GET")

	// Habilitar CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}).Handler(router)

	log.Println("Servidor escuchando en el puerto 3000...")
	log.Fatal(http.ListenAndServe(":3000", corsHandler))
}

func Login(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	row := db.QueryRow("SELECT id, username, password FROM users WHERE username = ?", user.Username)
	if err := row.Scan(&user.ID, &user.Username, &user.Password); err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusUnauthorized)
		return
	}

	// Verificar contraseña
	if user.Password != user.Password {
		http.Error(w, "Contraseña incorrecta", http.StatusUnauthorized)
		return
	}

	// Generar JWT
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		Username: user.Username,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Retornar el token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func Register(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", user.Username, user.Password)
	if err != nil {
		http.Error(w, "Error al registrar usuario", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func Dashboard(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	tokenString = tokenString[len("Bearer "):]

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Token inválido", http.StatusUnauthorized)
		return
	}

	row := db.QueryRow("SELECT username FROM users WHERE username = ?", claims.Username)
	var username string
	if err := row.Scan(&username); err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"username": username})
}

// Nuevo endpoint para obtener todos los usuarios
func GetUsers(w http.ResponseWriter, r *http.Request) {
	// Verificar JWT
	tokenString := r.Header.Get("Authorization")
	tokenString = tokenString[len("Bearer "):]

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "Token inválido", http.StatusUnauthorized)
		return
	}

	// Consultar todos los usuarios en la base de datos
	rows, err := db.Query("SELECT id, username FROM users")
	if err != nil {
		http.Error(w, "Error al obtener los usuarios", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username); err != nil {
			http.Error(w, "Error al procesar los usuarios", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	// Enviar la lista de usuarios como respuesta
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}