package httpx

import (
	"net/http"

	"github.com/Abdull247/KludgeServer/internal/respond"
)

func JSON(w http.ResponseWriter, status int, data any) {
	respond.JSON(w, status, data)
}

func Error(w http.ResponseWriter, status int, message string) {
	respond.Error(w, status, message)
}
