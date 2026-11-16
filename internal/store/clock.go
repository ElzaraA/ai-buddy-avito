package store

import (
	"sync/atomic"
	"time"
)

// Часовой пояс продукта: «сегодня» для дат отгулов и «Пульса» считается по Москве, а не по UTC
// контейнера (иначе отметка в 01:30 МСК попадала во «вчера»). Имя пояса — env TZ_NAME
// (по умолчанию Europe/Moscow). В образе без tzdata LoadLocation падает — тогда берём
// фиксированный UTC+3 (в Москве нет перехода на летнее время).

// DefaultTZName — пояс по умолчанию.
const DefaultTZName = "Europe/Moscow"

// mskFixed — запасной пояс, если базы часовых поясов нет.
var mskFixed = time.FixedZone("MSK", 3*60*60)

var location atomic.Pointer[time.Location]

func init() { location.Store(LoadLocation(DefaultTZName)) }

// LoadLocation загружает пояс по имени; пустое имя — Europe/Moscow; ошибка — фиксированный UTC+3.
func LoadLocation(name string) *time.Location {
	if name == "" {
		name = DefaultTZName
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	return mskFixed
}

// SetLocation задаёт часовой пояс продукта (из конфигурации TZ_NAME).
func SetLocation(name string) { location.Store(LoadLocation(name)) }

// Location — текущий часовой пояс продукта.
func Location() *time.Location { return location.Load() }

// Now — текущее время в поясе продукта.
func Now() time.Time { return time.Now().In(Location()) }

// LocalDate — дата YYYY-MM-DD в поясе продукта.
func LocalDate(t time.Time) string { return t.In(Location()).Format(dateLayout) }
