package main 

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Event struct {
	ID 				int 		`json:"id"`
	Title 			string		`json:"title"`
	Date 			string		`json:"date"`
	MaxParticipants int			`json:"max_participants"`
}

type User struct {
	ID    int 		`json:"id"`
	Name  string 	`json:"name"`
	Email string	`json:"email"`
}

type Booking struct {
	ID 		 int 	`json:"id"`
	EventID  int 	`json:"event_id"`
	UserID 	 int 	`json:"user_id"`
	BookedAt string `json:"booked_at"`
}

var bookingRequests chan Booking

var db *sql.DB

func main() {

	bookingRequests = make(chan Booking, 10)

	go processBookings()

	var err error 

	dsn := "postgres://postgres:postgres@localhost:5432/database"
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()
	if db.PingContext(ctx) != nil {
		log.Fatalf("Проблема с пингом БД: %v", db.PingContext(ctx))
	}
	log.Println("Успешное подключение к БД")

	r := chi.NewRouter()

	r.Post("/events", createEventHandler)
	r.Get("/events", listEventsHandler)
	r.Get("/events/{id}", getEventHandler)

	r.Post("/users", createUserHandler)
	r.Get("/users", listUsersHandler)

	r.Post("/bookings", createBookingHandler)

	r.Get("/events/{id}/participants", listParticipantsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Сервер запущен на порту %s", port)
	log.Fatal(http.ListenAndServe(":"+port,r))
}

func createEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var e Event 
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, "Некорректный JSON", http.StatusBadRequest)
		return
	}

	if e.Title == "" || e.Date == "" || e.MaxParticipants <= 0 {
		http.Error(w, "Отсутствуют обязательные поля", http.StatusBadRequest)
		return
	}

	query := "INSERT INTO events (title, date, max_participants) VALUES ($1, $2, $3) RETURNING id"
	err := db.QueryRowContext(ctx, query, e.Title, e.Date, e.MaxParticipants).Scan(&e.ID)
	if err != nil {
		http.Error(w, "Ошибка записи в базу", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(e)
}

func listEventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := db.QueryContext(ctx, "SELECT id, title, date, max_participants FROM events ORDER BY date")
	if err != nil {
		http.Error(w, "Ошибка запроса к базе", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event 
		if err := rows.Scan(&e.ID, &e.Title, &e.Date, &e.MaxParticipants); err != nil {
			http.Error(w, "Ошибка чтения даных", http.StatusInternalServerError)
			return
		}
		events = append(events, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func getEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var e Event
	query := "SELECT id, title, max_participants FROM events WHERE id=$1"
	err := db.QueryRowContext(ctx, query, id).Scan(&e.ID, &e.Title, &e.Date, &e.MaxParticipants)
	if err == sql.ErrNoRows {
		http.Error(w, "Событие не найдено", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Ошибка базы данных", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "Некорректный JSON", http.StatusBadRequest)
		return
	}

	if u.Name == "" || u.Email == "" {
		http.Error(w, "Отсутствуют обязательные поля", http.StatusBadRequest)
		return
	}

	query := "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id"
	err := db.QueryRowContext(ctx, query, u.Name, u.Email).Scan(&u.ID)
	if err != nil {
		http.Error(w, "Ошибка в записи в базу", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func listUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := db.QueryContext(ctx, "SELECT id, name, email FROM users ORDER BY id")
	if err != nil {
		http.Error(w, "Ошибка запроса к базе", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func createBookingHandler(w http.ResponseWriter, r *http.Request) {
	var b Booking
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, "Некорректный JSON", http.StatusBadRequest)
		return
	}

	if b.EventID == 0 || b.UserID == 0 {
		http.Error(w, "Нужны поля event_id и user_id", http.StatusBadRequest)
		return
	}

	select {
	case bookingRequests <- b:
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("{\"status\":\"Заявка на бронирование отправлена\"}"))
	default:
		http.Error(w, "Очередь бронирований переполнена", http.StatusTooManyRequests)
	}
}

func processBookings() {
	for b := range bookingRequests {
		ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
		defer cancel()

		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bookings WHERE event_id=$1", b.EventID).Scan(&count)
		if err != nil {
			log.Printf("Ошибка проверки бронирований: %v", err)
			continue
		}

		var max int
		err = db.QueryRowContext(ctx, "SELECT max_participants FROM events WHERE id=$1", b.EventID).Scan(&max)
		if err != nil {
			log.Printf("Ошибка чтения события: %v", err)
			continue
		}

		if count >= max {
			log.Printf("Событие %d заполнено (макс: %d)", b.EventID, max)
			continue
		}

		query := "INSERT INTO bookings (event_id, user_id) VALUES ($1, $2) RETURNING id, booked_at"
		err = db.QueryRowContext(ctx, query, b.EventID, b.UserID).Scan(&b.ID, &b.BookedAt)
		if err != nil {
			log.Printf("Ошибка добавления бронирования: %v", err)
			continue
		}
		
		log.Printf("Пользователь %d забронировал место на событии %d", b.UserID, b.EventID)

	}
}

func listParticipantsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eventID := chi.URLParam(r, "id")

	query := `
		SELECT u.id, u.name, u.email, b.booked_at
		FROM bookings b
		JOIN users u ON b.user_id = u.id 
		WHERE b.event_id = $1
		ORDER BY b.booked_at DESC;
		`

	rows, err := db.QueryContext(ctx, query, eventID)
	if err != nil {
		http.Error(w, "Ошибка запроса", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Participant struct {
		ID 		 int		`json:"id"`
		Name 	 string 	`json:"name"`
		Email 	 string 	`json:"email"`
		BookedAt string 	`json:"booked_at"`
	}

	var participants []Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.BookedAt); err != nil {
			http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
			return
		}
		participants = append(participants, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(participants)
}