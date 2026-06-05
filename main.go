package main

import (
	"os"
	"fmt"
	"log"
	"time"
	"errors"
	"strings"
	"net/http"
	"sync/atomic"
	"database/sql"
	"encoding/json"
	_ "github.com/lib/pq"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/iavishagupta/chirpy/internal/auth"
	"github.com/iavishagupta/chirpy/internal/database"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
	platform        string
	tokenSecret		string
	polkaAPIKey		string
}

//HEALTHZ
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

//MIDDLEWARE
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

//GET HITS
func (cfg *apiConfig) getHits(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<html>
								<body>
									<h1>Welcome, Chirpy Admin</h1>
									<p>Chirpy has been visited %d times!</p>
								</body>
								</html>`, cfg.fileServerHits.Load())))
}

//RESET HITS
func (cfg *apiConfig) resetHits(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden", nil)
		return
	}
	err := cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't delete users", err)
		return
	}

	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	cfg.fileServerHits.Store(0)
	w.Write([]byte(fmt.Sprintf("Hits: %d", cfg.fileServerHits.Load())))
}

//RESPOND WITH ERROR
func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

//RESPOND WITH JSON
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}

//REMOVE PROFANES
func removeProfanes(body string) string{
	words := strings.Split(body, " ")
	for i, word := range words {
		if (strings.ToLower(word) == "kerfuffle") || (strings.ToLower(word) == "sharbert") || (strings.ToLower(word) == "fornax"){
			words[i] = "****"
		}
	}

	fullStr := strings.Join(words, " ")
	return fullStr
}

//VALIDATE CHIRPS
func (cfg *apiConfig)handlerChirpsValidate(w http.ResponseWriter, r *http.Request) {
	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	type inputChirp struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
		Token    string	 `json:"token"`
	}

	
	input := inputChirp{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&input); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 400, "No JWT Found", err)
		return
	}

	user_id, err := auth.ValidateJWT(token, cfg.tokenSecret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	params := database.AddChirpsParams{
		Body:   input.Body,
		UserID: user_id,
	}

	const maxChirpLength = 140
	if len(params.Body) > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long", nil)
		return
	}
	params.Body = removeProfanes(params.Body)
	chirp, err := cfg.db.AddChirps(r.Context(), params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't add Chirps", err)
		return
	}
	respondWithJSON(w, 201, Chirp{
						ID:        chirp.ID,
						CreatedAt: chirp.CreatedAt,
						UpdatedAt: chirp.UpdatedAt,
						Body:      chirp.Body,
						UserID:    chirp.UserID,
					})
}

//CREATE USER
func (cfg *apiConfig)createUser(w http.ResponseWriter, r *http.Request){
	type AddUser struct{
		Password string `json:"password"`
		Email string `json:"email"`
	}

	type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	IsChirpyRed bool    `json:"is_chirpy_red"`
	}

	new_user := AddUser{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&new_user); err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
    	return	
	}

	hashedPassword, err := auth.HashPassword(new_user.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't hash password", err)
		return
	}

	user_res, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		Email:          new_user.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create user", err)
		return
    }

	user := User{
		ID: user_res.ID,
		CreatedAt: user_res.CreatedAt.Time,
		UpdatedAt: user_res.UpdatedAt.Time,
		Email: user_res.Email,
		IsChirpyRed: user_res.IsChirpyRed,
	}

	respondWithJSON(w, http.StatusCreated, user)
}

//GET ALL CHIRPS
func (cfg *apiConfig)ReturnAllChirps(w http.ResponseWriter, r *http.Request){
	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	id := r.URL.Query().Get("author_id")
	var auth_id uuid.UUID
	var err error
	
	sortIn := r.URL.Query().Get("sort")

	var outputChirps []database.Chirp
	if id == "" {
		if sortIn == "desc"{
			outputChirps, err = cfg.db.GetAllChirpsDesc(r.Context()) 
		}else{
			outputChirps, err = cfg.db.GetAllChirpsAsc(r.Context()) 
		}

		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Couldn't return chirps", err)
			return
		}
	} else {
		auth_id, err = uuid.Parse(id)

		if err != nil {
        respondWithError(w, http.StatusBadRequest, "Invalid author ID", err)
        return 
    	}

		if sortIn == "desc"{
			outputChirps, err = cfg.db.GetChirpsByAuthorDesc(r.Context(), auth_id) 
		}else{
			outputChirps, err = cfg.db.GetChirpsByAuthorAsc(r.Context(), auth_id) 
		}

		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Couldn't return chirps", err)
			return
		}
	}
		
	chirps := []Chirp{}
	for _, c := range outputChirps {
		chirps = append(chirps, Chirp{
			ID:        c.ID,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
			Body:      c.Body,
			UserID:    c.UserID,
		})
	}

respondWithJSON(w, http.StatusOK, chirps)
}

//GET SINGLE CHIRP BY CHIRP ID
func (cfg *apiConfig)HandlerGetChirp(w http.ResponseWriter, r *http.Request){
	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	chirp_id := r.PathValue("chirpID")

	uuid_, err := uuid.Parse(chirp_id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse ChirpID", err)
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), uuid_)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "Chirp not found", err)
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Couldn't get chirp", err)
		return
	}

	respondWithJSON(w, 200, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})

}

//LOGIN USER
func(cfg *apiConfig)loginUser(w http.ResponseWriter, r *http.Request){
	type LoginUser struct{
		Password string `json:"password"`
		Email string `json:"email"`
		ExpiresInSeconds int `json:"expires_in_seconds"`
	}

	var user LoginUser
	decoder := json.NewDecoder(r.Body)
	if err:=decoder.Decode(&user); err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode credentials", err)
		return
	}

	if user.ExpiresInSeconds == 0 || user.ExpiresInSeconds > 3600 {
		user.ExpiresInSeconds = 3600
	}

	db_cred, err := cfg.db.AuthUser(r.Context(), user.Email)
	if err != nil{
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	ok, err := auth.CheckPasswordHash(user.Password, db_cred.HashedPassword)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Can't authenticate", err)
		return
	}
	if !ok{
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	// 1. Generate the Access Token (JWT)
	// This is the one the client uses in the Authorization header for most requests
	accessToken, err := auth.MakeJWT(db_cred.ID, cfg.tokenSecret, time.Duration(user.ExpiresInSeconds) * time.Second)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can't create a JWT", err)
		return
	}

	// 2. Generate the Refresh Token (Random Hex String)
	refreshTokenString := auth.MakeRefreshToken()

	// 3. Save the Refresh Token to the database
	refresh_token, err := cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshTokenString,
		UserID:    db_cred.ID,
		ExpiresAt: time.Now().Add(24 * 60 * time.Hour), // 60 days
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Cant create Refresh Token", err)
		return
	}

	// 4. Respond with both
	respondWithJSON(w, 200, struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
		IsChirpyRed  bool      `json:"is_chirpy_red"`
	}{
		ID:           db_cred.ID,
		CreatedAt:    db_cred.CreatedAt.Time,
		UpdatedAt:    db_cred.UpdatedAt.Time,
		Email:        db_cred.Email,
		Token:        accessToken,      
		RefreshToken: refresh_token.Token, 
		IsChirpyRed:  db_cred.IsChirpyRed,
	})
}

//GENERATE NEW ACCESS TOKEN
func (cfg *apiConfig) handlerNewAccessToken(w http.ResponseWriter, r *http.Request) {
	refresh_token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find token", err)
		return
	}

	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), refresh_token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	new_access_token, err := auth.MakeJWT(
		user.ID, 
		cfg.tokenSecret, 
		time.Hour,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Can't create a JWT", err)
		return
	}

	respondWithJSON(w, http.StatusOK, struct {
		Token string `json:"token"`
	}{
		Token: new_access_token,
	})
}

// REVOKE REFRESH TOKEN
func (cfg *apiConfig)revokeRefreshToken(w http.ResponseWriter, r *http.Request){
	refresh_token, err := auth.GetBearerToken(r.Header)
	if err!= nil{
		respondWithError(w, http.StatusInternalServerError, "Error revoking Refresh Token", err)
		return
	}
	if err := cfg.db.RevokeRefreshToken(r.Context(), refresh_token); err!= nil{
		respondWithError(w, http.StatusInternalServerError, "Error revoking Refresh Token", err)
		return
	}
	respondWithJSON(w, 204, nil)
	return
}

//UPDATE DETAILS
func (cfg *apiConfig)handlerUpdateDetails(w http.ResponseWriter, r *http.Request){

	var userNewParams database.CreateUserParams

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&userNewParams); err != nil{
		respondWithError(w, http.StatusInternalServerError, "Error Decoding New Parameters", err)
		return
	}

	hashed_password, err := auth.HashPassword(userNewParams.HashedPassword)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Error Hashing Password", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	user_id, err := auth.ValidateJWT(token, cfg.tokenSecret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	if err := cfg.db.UpdateUserDetails(r.Context(), database.UpdateUserDetailsParams{
		Email: userNewParams.Email,
		HashedPassword: hashed_password,
		ID: user_id,
	}); err!=nil{
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	respondWithJSON(w, 200, struct{Email string`json:"email"`}{
								Email: userNewParams.Email,
							})
}

//DELETE CHIRP BY CHIRP ID
func (cfg *apiConfig)handlerDeleteChirps(w http.ResponseWriter, r *http.Request){
	chirp_id := r.PathValue("chirpID")
	uuid_Chirp, err := uuid.Parse(chirp_id)
	if err != nil {
		respondWithError(w, 403, "Couldn't parse ChirpID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "No JWT Found", err)
		return
	}

	user_id, err_ := auth.ValidateJWT(token, cfg.tokenSecret)
	if err_ != nil {
		respondWithError(w, 403, "Unauthorized", err_)
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), uuid_Chirp)
	if err != nil {
		respondWithError(w, 404, "Chirp Not Found", err_)
		return
	}

	if chirp.UserID != user_id{
		respondWithError(w, 403, "Unauthorized", errors.New("Trying to access unauthorized Chirp"))
		return		
	}

	if err := cfg.db.DeleteChirpByID(r.Context(), uuid_Chirp); err != nil{
		respondWithError(w, 404, "Chirp Not Found", err)
	}

	respondWithJSON(w, 204, nil)
}

//UPDATE MEMBERSHIP
func (cfg *apiConfig)handlerUpdateMembership(w http.ResponseWriter, r *http.Request){
    type parameters struct {
        Event string `json:"event"`
        Data  struct {
            UserID string `json:"user_id"`
        } `json:"data"`
    }

	polkakey, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, 401, "Polka Key Not Found", err)
		return
	}

	if polkakey != cfg.polkaAPIKey {
		w.WriteHeader(401)
		return
	}
	params := parameters{}
	decoder := json.NewDecoder(r.Body)
	if err:=decoder.Decode(&params); err!=nil{
		respondWithError(w, http.StatusInternalServerError, "Error Decoding Polka Request", err)
		return
	}

	if params.Event != "user.upgraded"{
		w.WriteHeader(204)
		return
	}

	user_uuid, err := uuid.Parse(params.Data.UserID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error Parsing UserID", err)
		return
	}

	if err := cfg.db.UpdateMembership(r.Context(), user_uuid); err != nil {
		respondWithError(w, 404, "User Not Found", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	const filepathRoot = "."
	const port = "8080"
	
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL must be set")
	}

	dbConn, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database: %s", err)
	}
	dbQueries := database.New(dbConn)

	apiCfg := apiConfig{
		fileServerHits: atomic.Int32{},
		db:             dbQueries,
		platform:       os.Getenv("PLATFORM"),
		tokenSecret:    os.Getenv("TOKEN_SECRET"),
		polkaAPIKey:	os.Getenv("POLKA_KEY"),
	}

	mux := http.NewServeMux()

	handler := http.StripPrefix("/app/", http.FileServer(http.Dir(filepathRoot)))

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))

	mux.HandleFunc("GET /admin/metrics", apiCfg.getHits)
	
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.HandlerGetChirp)
	mux.HandleFunc("GET /api/chirps", apiCfg.ReturnAllChirps)

	mux.HandleFunc("POST /admin/reset", apiCfg.resetHits)

	mux.HandleFunc("POST /api/users", apiCfg.createUser)
	mux.HandleFunc("POST /api/chirps",apiCfg.handlerChirpsValidate)   
	mux.HandleFunc("POST /api/login", apiCfg.loginUser)
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerNewAccessToken)
	mux.HandleFunc("POST /api/revoke", apiCfg.revokeRefreshToken)
	mux.HandleFunc("POST /api/polka/webhooks", apiCfg.handlerUpdateMembership)

	mux.HandleFunc("PUT /api/users", apiCfg.handlerUpdateDetails)

	mux.HandleFunc("DELETE /api/chirps/{chirpID}", apiCfg.handlerDeleteChirps)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
}