package mirvpgl

import (
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/olahol/melody.v1"
)

// HLAESession wraps some session-related functions and is also a melody.Session wrapper
type HLAESession struct {
	*melody.Session
}

// UUID returns unified id for this session
func (s HLAESession) UUID() uuid.UUID {
	uuidIfs, ok := s.Get("uuid")
	if !ok {
		return uuid.Nil
	}
	return uuidIfs.(uuid.UUID)
}

// SetUUID set a new unified id for this session
func (s HLAESession) SetUUID(uuid uuid.UUID) {
	s.Set("uuid", uuid)
}

// UUIDAsLogField creates a zap.Field representing session uuid
func (s HLAESession) UUIDAsLogField() zap.Field {
	return zap.String("session_uuid", s.UUID().String())
}
