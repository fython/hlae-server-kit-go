package mirvpgl

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"

	"gopkg.in/olahol/melody.v1"
)

const (
	mirvPglVersion = 2
)

type HLAEServerArguments struct {
	Logger *zap.Logger
}

// HLAEServer Main struct
type HLAEServer struct {
	logger          *zap.Logger
	melody          *melody.Melody
	sessions        []HLAESession
	eventSerializer *gameEventUnserializer

	handlers          []func(HLAEServerCommand)
	camHandlers       []func(*CamData)
	eventHandlers     []func(*GameEventData)
	levelInitHandlers []func(string)
}

// New Get new instance of HLAEServer
func New(args HLAEServerArguments) (srv *HLAEServer, err error) {
	if args.Logger == nil {
		args.Logger, err = zap.NewDevelopmentConfig().Build()
		if err != nil {
			err = fmt.Errorf("failed to init logger: %w", err)
			return
		}
	}
	srv = &HLAEServer{
		logger:   args.Logger,
		melody:   melody.New(),
		sessions: make([]HLAESession, 0),
	}
	srv.eventSerializer = newGameEventUnserializer(enrichments)

	srv.melody.HandleConnect(func(session *melody.Session) {
		hs := HLAESession{Session: session}
		if hs.UUID() == uuid.Nil {
			hs.SetUUID(uuid.New())
		}
		srv.websocketHandleConnect(hs)
	})
	srv.melody.HandleDisconnect(func(session *melody.Session) {
		hs := HLAESession{Session: session}
		srv.websocketHandleDisconnect(hs)
	})
	srv.melody.HandleMessageBinary(func(session *melody.Session, data []byte) {
		hs := HLAESession{Session: session}
		srv.websocketHandleMessageBinary(hs, data)
	})

	return srv, nil
}

func (h *HLAEServer) websocketHandleConnect(s HLAESession) {
	h.sessions = append(h.sessions, s)
	h.logger.Info("HLAE WebSocket client connected.",
		zap.Int("current_sessions_count", len(h.sessions)))
}

func (h *HLAEServer) websocketHandleDisconnect(s HLAESession) {
	// Remove session from session slice
	for idx, v := range h.sessions {
		if v.Session == s.Session {
			newSessions := make([]HLAESession, len(h.sessions)-1)
			newSessions = append(h.sessions[:idx], h.sessions[idx+1:]...)
			h.sessions = newSessions
			h.logger.Info("HLAE Websocket client disconnected.",
				zap.Int("current_sessions_count", len(h.sessions)))
			return
		}
	}
}

func (h *HLAEServer) websocketHandleMessageBinary(s HLAESession, data []byte) {
	buf := bytes.NewBuffer(data)
	cmdStr, err := buf.ReadString(nullStr)
	if err != nil {
		if err == io.EOF {
			h.logger.Debug("EOF", s.UUIDAsLogField())
		} else {
			h.logger.Error("failed to read string from buffer",
				s.UUIDAsLogField(), zap.Error(err))
		}
		return
	}
	cmd := HLAEServerCommand(strings.ReplaceAll(cmdStr, string(nullStr), ""))
	h.logger.Debug("received command",
		s.UUIDAsLogField(), zap.String("cmd", cmdStr))
	switch cmd {
	case ServerCommandHello:
		h.logger.Info("HLAE Client connection established.",
			s.UUIDAsLogField())
		var version uint32
		if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
			h.logger.Error("failed to read version message buffer",
				s.UUIDAsLogField(), zap.Error(err))
			return
		}
		h.logger.Info("Got version message",
			s.UUIDAsLogField(), zap.Uint32("version", version))
		if version != mirvPglVersion {
			h.logger.Error("Client version is mismatched. exited",
				s.UUIDAsLogField(),
				zap.Uint32("expected_version", mirvPglVersion))
			return
		}

		h.TransBegin()
		s.WriteBinary(commandToByteSlice("mirv_pgl events enrich clientTime 1"))
		for eventName, v := range enrichments {
			for enrichName, e := range v {
				for _, er := range e.GetEnrichment() {
					cmd := fmt.Sprintf(`mirv_pgl events enrich eventProperty "%s" "%s" "%s"`,
						er, eventName, enrichName)
					s.WriteBinary(commandToByteSlice(cmd))
				}
			}
		}
		s.WriteBinary(commandToByteSlice("mirv_pgl events enabled 1"))
		s.WriteBinary(commandToByteSlice("mirv_pgl events useCache 1"))
		h.TransEnd()

		h.handleRequest(cmd)
	case ServerCommandDataStop:
		h.logger.Info("HLAE Client stopped sending data.", s.UUIDAsLogField())
		h.handleRequest(cmd)
	case ServerCommandDataStart:
		h.logger.Info("HLAE Client started sending data.", s.UUIDAsLogField())
		h.handleRequest(cmd)
	case ServerCommandLevelInit:
		mapName, err := buf.ReadString(nullStr)
		if err != nil {
			h.logger.Error("failed to read levelInit message buffer",
				s.UUIDAsLogField(), zap.Error(err))
			return
		}
		h.logger.Info("level init",
			s.UUIDAsLogField(), zap.String("map", mapName))
		h.handleLevelInitRequest(mapName)
	case ServerCommandLevelShutdown:
		h.logger.Info("received levelShutdown", s.UUIDAsLogField())
		h.handleRequest(cmd)
	case ServerCommandCam:
		camData := &CamData{}
		if err := binary.Read(buf, binary.LittleEndian, camData); err != nil {
			h.logger.Info("failed to parse cam message buffer",
				s.UUIDAsLogField(), zap.Error(err))
			return
		}
		h.handleCamRequest(camData)
	case ServerCommandGameEvent:
		ev, err := h.eventSerializer.Unserialize(buf)
		if err != nil {
			h.logger.Error("failed to parse event desc",
				s.UUIDAsLogField(), zap.Error(err))
			return
		}
		h.logger.Debug("EVENT", s.UUIDAsLogField(), zap.Any("event", ev))
		h.handleEventRequest(ev)
	default:
		h.logger.Warn("unknown message", s.UUIDAsLogField(), zap.String("cmd", cmdStr))
		h.handleRequest(cmd)
	}
}

// ServeHTTP implements http.Handler interfaces
func (h *HLAEServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.melody.HandleRequest(w, r); err != nil {
		h.logger.Error("failed to handle request",
			zap.String("client", r.RemoteAddr),
			zap.Error(err))
	}
}

// BroadcastRCON broadcast command
func (h *HLAEServer) BroadcastRCON(cmd string) error {
	command := commandToByteSlice(cmd)
	if err := h.melody.BroadcastBinary(command); err != nil {
		return err
	}
	return nil
}

// SendRCON Send RCON to specific client
func (h *HLAEServer) SendRCON(k int, cmd string) error {
	if len(h.sessions)-1 < k {
		command := commandToByteSlice(cmd)
		if err := h.melody.BroadcastBinary(command); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("out of index slice")
}

// RegisterHandler to handle each requests
func (h *HLAEServer) RegisterHandler(handler func(HLAEServerCommand)) {
	h.handlers = append(h.handlers, handler)
	h.logger.Debug("registered handler",
		zap.Int("active_handlers", len(h.handlers)))
}

// RegisterCamHandler to handle each requests
func (h *HLAEServer) RegisterCamHandler(handler func(*CamData)) {
	h.camHandlers = append(h.camHandlers, handler)
	h.logger.Debug("registered camera handler",
		zap.Int("active_handlers", len(h.camHandlers)))
}

// RegisterEventHandler to handle each requests
func (h *HLAEServer) RegisterEventHandler(handler func(*GameEventData)) {
	h.eventHandlers = append(h.eventHandlers, handler)
	h.logger.Debug("registered event handler",
		zap.Int("active_handlers", len(h.eventHandlers)))
}

func (h *HLAEServer) RegisterLevelInitHandler(handler func(string)) {
	h.levelInitHandlers = append(h.levelInitHandlers, handler)
	h.logger.Debug("registered level init handler",
		zap.Int("active_handlers", len(h.levelInitHandlers)))
}

func (h *HLAEServer) handleRequest(cmd HLAEServerCommand) {
	for i := 0; i < len(h.handlers); i++ {
		go h.handlers[i](cmd)
	}
}

func (h *HLAEServer) handleCamRequest(cam *CamData) {
	for i := 0; i < len(h.handlers); i++ {
		go h.camHandlers[i](cam)
	}
}

func (h *HLAEServer) handleEventRequest(ev *GameEventData) {
	for i := 0; i < len(h.eventHandlers); i++ {
		go h.eventHandlers[i](ev)
	}
}

func (h *HLAEServer) handleLevelInitRequest(mapName string) {
	for i := 0; i < len(h.levelInitHandlers); i++ {
		go h.levelInitHandlers[i](mapName)
	}
}

// TransBegin Start transaction
func (h *HLAEServer) TransBegin() error {
	length := len("transBegin") + 1 // "transBegin" + nullStr
	command := make([]byte, 0, length)
	command = append(command, []byte("transBegin")...)
	command = append(command, nullStr)
	return h.melody.BroadcastBinary(command)
}

// TransEnd End transaction
func (h *HLAEServer) TransEnd() error {
	length := len("transEnd") + 1 // "transEnd" + nullStr
	command := make([]byte, 0, length)
	command = append(command, []byte("transEnd")...)
	command = append(command, nullStr)
	return h.melody.BroadcastBinary(command)
}
