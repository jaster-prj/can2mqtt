package persistence

type RouteDirection int

type Route struct {
	CanID     string         `json:"canid"`
	Topic     string         `json:"topic"`
	Direction RouteDirection `json:"direction"`
	Converter *string        `json:"converter,omitempty"`
}
