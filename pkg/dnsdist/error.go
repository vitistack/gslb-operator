package dnsdist

import "errors"

var (
	ErrCouldNotParseAddr         = errors.New("could not parse address")
	ErrWhileCreatingConnection   = errors.New("could not create server connection")
	ErrCouldNotWrite             = errors.New("no write on connection")
	ErrCouldNotRead              = errors.New("no read on connection")
	ErrFatalRandRead             = errors.New("init of client nonce failed irrecoverably")
	ErrUnSuccessFullBase64Decode = errors.New("could not decode base64 encoded string")
	ErrReceivedInvalidNonce      = errors.New("received invalid nonce")
	ErrInvalidNoncePair          = errors.New("invalid nonce client/server pair")
	ErrCouldNotSendCommand       = errors.New("could not send command")
	ErrHandShakeNotValid         = errors.New("handshake failed")
	ErrCouldNotDecrypt            = errors.New("could not decrypt")
)
