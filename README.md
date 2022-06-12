# hlae-server-kit-go

> This is a personal fork from [FlowingSPDG/HLAE-Server-GO](https://github.com/FlowingSPDG/HLAE-Server-GO).

HLAE Server Kit with Go implementation

# About
HLAE `mirv_pgl` command server implementation with Go.  
This package helps you to handle `mirv_pgl` command and their data.

## mirv_pgl
`mirv_pgl` supports to remote-control CS:GO client by using WebSocket.  
you can handle client's camera information(position,rotation,fov,time) and game-events, and you can send commands like RCON.

Official NodeJS code : https://github.com/advancedfx/advancedfx/blob/master/misc/mirv_pgl_test/server.js


## Usage
see [examples](https://github.com/fython/hlae-server-kit-go/blob/master/examples/main.go).  
- 1,Launch HLAE Server by ``go run main.go``.  
- 2,Launch CSGO with HLAE.
- 3,type following commands below:  
```
mirv_pgl url "ws://localhost:65535/";
mirv_pgl start;
mirv_pgl datastart;
```

once CS:GO client succeed to connect HLAE Server, you can send commands by typing window.
