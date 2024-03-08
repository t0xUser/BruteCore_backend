package repository

import (
	"errors"
	"time"

	"api.brutecore/libs/lib_env"
	"api.brutecore/libs/lib_jwt"
)

type AUTHRepositoryLayer struct {
	conf *lib_env.Config
	jwt  *lib_jwt.TJWT
}

type MLoginData struct {
	UserName string `json:"username" validate:"max=64"`
	Password string `json:"password" validate:"max=20"`
}

func NewAUTHRepositoryLayer(c *lib_env.Config, j *lib_jwt.TJWT) *AUTHRepositoryLayer {
	return &AUTHRepositoryLayer{
		conf: c,
		jwt:  j,
	}
}

func (r *AUTHRepositoryLayer) LoginInDataBase(lp *MLoginData) (*lib_jwt.TokenPair, error) {
	if r.conf.Auth.AdminLogin == lp.UserName && r.conf.Auth.AdminPassword == lp.Password {
		claim := lib_jwt.JWTClaim{"role_name": "ADMIN"}
		return r.jwt.GenerateTokenPair(&claim)
	} else if r.conf.Auth.ViewerLogin == lp.UserName && r.conf.Auth.ViewerPassword == lp.Password {
		claim := lib_jwt.JWTClaim{"role_name": "VIEWER"}
		return r.jwt.GenerateTokenPair(&claim)
	} else {
		return nil, errors.New("Неверный логин или пароль")
	}
}

func (r *AUTHRepositoryLayer) LogoutUser(token string) error {
	claim, _ := r.jwt.ParseToken(&token)
	var timely time.Time
	if r.conf.Jwt.AccessTime > r.conf.Jwt.RefreshTime {
		timely = claim.GetIAT().Add(r.conf.Jwt.AccessTime)
	} else {
		timely = claim.GetIAT().Add(r.conf.Jwt.RefreshTime)
	}

	lib_jwt.BlackList.AddToBlackList(claim.GetUUID(), timely)
	return nil
}

func (r *AUTHRepositoryLayer) CheckTokenMiddleware(token string) error {
	if token == "" {
		return errors.New("Invalid Token")
	}

	claim, err := r.jwt.ParseToken(&token)
	if err != nil {
		return err
	}

	if claim.GetUUID() == "" {
		return errors.New("UUID is undefined")
	}

	if claim.GetSUB() != "access" {
		return errors.New("Token is not access")
	}

	if claim.GetIAT() == nil {
		return errors.New("IAT is undefined")
	}

	if _, ok := lib_jwt.BlackList[claim.GetUUID()]; ok {
		return errors.New("Token is deactivated")
	}

	return nil
}

func (r *AUTHRepositoryLayer) RefreshToken(p *lib_jwt.TokenPair) (*lib_jwt.TokenPair, error) {
	aClaim, _ := r.jwt.ParseToken(p.AccessToken)
	rClaim, err := r.jwt.ParseToken(p.RefreshToken)
	if err != nil {
		return nil, err
	}

	if rClaim.GetSUB() != "refresh" {
		return nil, errors.New("Wrong refresh Token")
	}

	if aClaim.GetUUID() != rClaim.GetUUID() {
		return nil, errors.New("Different UUID")
	}

	UUID := aClaim.GetUUID() // Так сделано специально, поскольку после генерации UUID меняется, поскольку мы кидаем указатель
	pair, err := r.jwt.GenerateTokenPair(aClaim)
	if err != nil {
		return nil, err
	}

	var timely time.Time
	if rClaim.GetEXP().After(*aClaim.GetEXP()) {
		timely = *rClaim.GetEXP()
	} else {
		timely = *aClaim.GetEXP()
	}

	lib_jwt.BlackList.AddToBlackList(UUID, timely)
	return pair, nil
}

func (r *AUTHRepositoryLayer) CheckAdminRole(token string) error {
	claim, err := r.jwt.ParseToken(&token)
	if err != nil {
		return err
	}

	role := claim.GetValue("role_name").(string)
	if role != "ADMIN" {
		return errors.New("Недостаточно прав для выполнения данной операции")
	}
	return nil
}
