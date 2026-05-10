package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	"github.com/jaster-prj/can2mqtt/common"
	"github.com/jaster-prj/can2mqtt/persistence"
)

type Updater struct {
	routing     *Routing
	persistence *persistence.Persistence
	callbacks   []func([]Route, []Route)
}

func NewUpdate() *Updater {
	return &Updater{
		routing:   nil,
		callbacks: []func([]Route, []Route){},
	}
}

func (u *Updater) WithRouting(routing *Routing) *Updater {
	u.routing = routing
	return u
}

func (u *Updater) WithPersistence(persistence *persistence.Persistence) *Updater {
	u.persistence = persistence
	if persistence != nil {
		configData, err := persistence.ReadRoutes()
		if err != nil {
			slog.Error("Failed to read routes from persistence", "error", err)
		} else {
			u.ConfigUpdate(configData)
		}
	}
	return u
}

func (u *Updater) RegisterCallback(callback func([]Route, []Route)) {
	u.callbacks = append(u.callbacks, callback)
}

func (u *Updater) ConfigUpdate(config []byte) {
	if err := u.checkPersistence(config); err == nil {
		slog.Debug("Config update skipped due to checksum match")
		return
	}
	var routings []Route
	if err := json.Unmarshal(config, &routings); err != nil {
		slog.Error("Unmarshal config error", "config", string(config), "error", err)
		return
	}
	defer u.routing.UpdateRoutes(routings)
	addRoute, delRoute, err := u.routing.CompareRoutes(routings)
	if err != nil {
		slog.Error("ComperRoutes error", "error", err)
		return
	}
	u.Inform(addRoute, delRoute)
	if u.persistence != nil {
		if err := u.persistence.WriteRoutes(config); err != nil {
			slog.Error("Failed to persist routes", "error", err)
		}
	}
}

func (u *Updater) Inform(addRoute []Route, delRoute []Route) {
	for _, callback := range u.callbacks {
		callback(addRoute, delRoute)
	}
}

func (u *Updater) checkPersistence(config []byte) error {
	if u.persistence == nil {
		return common.ErrPersistenceNotInitialized
	}
	hashPersistence, err := u.persistence.GetChecksum()
	if err != nil {
		return err
	}
	hash := sha256.Sum256(config)
	hashString := hex.EncodeToString(hash[:])
	if hashString != hashPersistence {
		return common.ErrPersistenceChecksumMismatch
	}
	return nil
}
