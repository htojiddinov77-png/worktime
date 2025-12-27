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

func (p *password) Matches(plainTextPassword string) (bool, error) {
	if len(p.hash) == 0 {
		return false, errors.New("missing password hash")
	}
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plainTextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

type User struct {
	Id           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash password  `json:"-"`
	Role         string    `json:"role"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type AdminUserUpdate struct {
	IsActive *bool
	Role *string
}

type PostgresUserStore struct {
	db *sql.DB
}

func NewPostgresUserStore(db *sql.DB) *PostgresUserStore {
	return &PostgresUserStore{db: db}
}

type UserStore interface {
	CreateUser(*User) error
	GetUserById(id int64) (*User, error)
	GetUserByEmail(email string) (*User, error)
	UpdatePasswordPlain(userId int64, newPassword string) error
	UpdateUser(user *User) error
	AdminUserUpdate(id int64, in AdminUserUpdate) error
	ListUsers() ([]User, error)
}

func (pg *PostgresUserStore) CreateUser(user *User) error {
	query := `
		INSERT INTO users (name, email, password_hash, is_active, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, role, created_at;
	`

	err := pg.db.QueryRow(
		query,
		user.Name,
		user.Email,
		user.PasswordHash.hash,
		user.IsActive,
	).Scan(&user.Id,&user.Role, &user.CreatedAt)

	if err != nil {
		return err
	}
	return nil
}

func (pg *PostgresUserStore) GetUserById(id int64) (*User, error) {
	user := &User{}

	query := `
		SELECT id, name, email, password_hash, role, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var pwHash string
	err := pg.db.QueryRow(query, id).Scan(
		&user.Id,
		&user.Name,
		&user.Email,
		&pwHash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (pg *PostgresUserStore) GetUserByEmail(email string) (*User, error) {
	user := &User{}

	query := `
		SELECT id, name, email, password_hash, role, is_active, created_at
		FROM users
		WHERE email = $1
	`

	err := pg.db.QueryRow(query, email).Scan(
		&user.Id,
		&user.Name,
		&user.Email,
		&user.PasswordHash.hash,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (pg *PostgresUserStore) UpdateUser(user *User) error {
	query := `
		UPDATE users
		SET name = $1, email = $2, password_hash = $3,  updated_at = NOW()
		WHERE id = $4
	`

	_, err := pg.db.Exec(query,
		user.Name,
		user.Email,
		user.PasswordHash.hash,
		user.Id,
	)
	return err
}



func (pg *PostgresUserStore) AdminUserUpdate(id int64, upd AdminUserUpdate) error {
	if upd.IsActive == nil && upd.Role == nil {
		return errors.New("no fields to update")
	}

	query := `
	UPDATE users
	SET 
		is_active = COALESCE($2, is_active),
		role = COALESCE($3, role)
		updated_at = NOW(),
	WHERE id = $1`

	res,err := pg.db.Exec(query, id, upd.IsActive, upd.Role)
	if err != nil {
		return  err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (pg *PostgresUserStore) ListUsers() ([]User, error) {
	query := `
        SELECT id, name, email, role, is_active, created_at, updated_at
        FROM users
        ORDER BY id ASC
    `
	rows, err := pg.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.Id,
			&u.Name,
			&u.Email,
			&u.Role,
			&u.IsActive,
			&u.CreatedAt,
			&u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (pg *PostgresUserStore) UpdatePasswordPlain(userId int64, newPassword string) error {
	var p password
	err := p.Set(newPassword)
	if err != nil {
		return err
	}

	hashBytes := p.hash
	query := `UPDATE users
	SET password_hash = $1, updated_at = NOW()
	WHERE id = $2`

	res, err := pg.db.Exec(query, hashBytes, userId)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
