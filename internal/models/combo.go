package models

import "time"

type Combo struct {
	ID        int64         `json:"id" db:"id"`
	Name      string        `json:"name" db:"name"`
	Targets   []ComboTarget `json:"targets" db:"targets"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt time.Time     `json:"updated_at" db:"updated_at"`
}

type ComboTarget struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Priority int    `json:"priority"`
}

type ComboService interface {
	Create(combo *Combo) error
	Read(id int64) (*Combo, error)
	Update(combo *Combo) error
	Delete(id int64) error
	List() ([]*Combo, error)
}
