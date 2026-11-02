package http

import (
	"time"

	"github.com/MertzZzzz/OpenAudio/api/internal/domain"
	"github.com/MertzZzzz/OpenAudio/api/internal/usecase/auth"
)

type identityDTO struct {
	Provider  string    `json:"provider"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"linkedAt"`
}

type userDTO struct {
	ID            string        `json:"id"`
	Email         string        `json:"email"`
	DisplayName   string        `json:"displayName"`
	Role          domain.Role   `json:"role"`
	AvatarURL     *string       `json:"avatarUrl"`
	EmailVerified bool          `json:"emailVerified"`
	HasPassword   bool          `json:"hasPassword"`
	Identities    []identityDTO `json:"identities,omitempty"`
	CreatedAt     time.Time     `json:"createdAt"`
}

func toUserDTO(u *domain.User, ids []domain.Identity) userDTO {
	d := userDTO{
		ID: u.ID.String(), Email: u.Email, DisplayName: u.DisplayName, Role: u.Role, AvatarURL: u.AvatarURL,
		EmailVerified: u.EmailVerified(), HasPassword: u.HasPassword(), CreatedAt: u.CreatedAt,
	}
	if ids != nil {
		d.Identities = make([]identityDTO, len(ids))
		for i, id := range ids {
			d.Identities[i] = identityDTO{Provider: string(id.Provider), Email: id.Email, CreatedAt: id.CreatedAt}
		}
	}
	return d
}

type tokenPairDTO struct {
	AccessToken  string  `json:"accessToken"`
	TokenType    string  `json:"tokenType"`
	ExpiresIn    int     `json:"expiresIn"`
	RefreshToken string  `json:"refreshToken"`
	User         userDTO `json:"user"`
}

func toTokenPair(s *auth.Session, now time.Time) tokenPairDTO {
	return tokenPairDTO{
		AccessToken: s.AccessToken, TokenType: "Bearer", ExpiresIn: int(s.AccessExpiresAt.Sub(now).Seconds()),
		RefreshToken: s.RefreshToken, User: toUserDTO(s.User, nil),
	}
}
