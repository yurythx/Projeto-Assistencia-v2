package transport

import apperrors "github.com/yurythx/projeto-nexus/internal/domain/errors"

func httputilValidation(msg string) error { return apperrors.Validation(msg) }
