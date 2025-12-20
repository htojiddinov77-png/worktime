package store

import (
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type password struct {
	plainText *string
	hash      []byte
}

func (p *password) Set(plainTextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainTextPassword), 12)
	if err != nil {
		return err
	}

	p.plainText = &plainTextPassword
	p.hash = hash
	return nil
}

func (p *password) Matches(plainTextPassword string) (bool,error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plainTextPassword))
	if err != nil {
		switch  {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true,nil
}

type User struct {
	Id           int64    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash password  `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{
		db: db,
	}
}

type UserStore interface{
	CreateUser(*User) error
	GetUserById(id int64) (*User, error)
	GetUserByEmail(email string) (*User, error)
	UpdateUser(user *User) error
	DisableUser(id int64) error
}


func (pg *PostgresUserStore) CreateUser(user *User) error {
	query := `
	INSERT INTO users (name, email, password_hash, role, created_at, updated_at)
	VALUES($1, $2, $3, $4, NOW(), NOW())
	RETURNING id`

	err := pg.db.QueryRow(query, user.Name, user.Email, user.PasswordHash.hash, user.Role, user.CreatedAt, user.UpdatedAt).Scan(&user.Id)

	if err != nil {
		return nil
	}

	return nil
}

func (pg *PostgresUserStore) GetUserById(id int64) (*User,error) {
	user := &User{}

	query := `
	SELECT id, name, email, password_hash, role, created_at, updated_at
	FROM users
	WHERE id = $1`
	
	row := pg.db.QueryRow(query, id)
	err := row.Scan(
		&user.Id,
		&user.Name,
		&user.Email,
		&user.PasswordHash.hash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)


	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return user,nil
}

func (pg *PostgresUserStore) GetUserByEmail(email string) (*User, error) {
	user := &User{}
	query := `SELECT id, name, email, password_hash, role, created_at, updated_at
	FROM users
	WHERE email = $1`

	row := pg.db.QueryRow(query, email)
	err := row.Scan(
		&user.Id,
		&user.Name,
		&user.Email,
		&user.PasswordHash.hash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return user,nil
}

func (pg *PostgresUserStore) UpdateUser(user *User) error {

	query := `UPDATE users
	SET name = $1, email = $2, password_hash = $3, role = $4, updated_at = NOW()
	WHERE id = $5`
	_, err := pg.db.Exec(query, user.Name, user.Email, user.PasswordHash.hash, user.Role, user.Id)
	if err != nil {
		return err
	}
	return nil
}

func (pg *PostgresUserStore) DisableUser(id int64) error {
	query := `UPDATE users
	SET is_active = false
	WHERE id = $1`

	_, err := pg.db.Exec(query, id)
	if err != nil {
		return err
	}
	return nil
}


