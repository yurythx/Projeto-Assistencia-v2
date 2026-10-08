package transport

import apperrors "github.com/yurythx/projeto-assistencia-v2/internal/domain/errors"

func httputilValidation(msg string) error { return apperrors.Validation(msg) }
