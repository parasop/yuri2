package api

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/zekroTJA/yuri2/internal/discordbot"
	"github.com/zekroTJA/yuri2/internal/logger"
	"github.com/zekroTJA/yuri2/internal/static"
)

type getTokenResponse struct {
	Token  string    `json:"token"`
	UserID string    `json:"user_id"`
	Expire time.Time `json:"expires"`
}

type listResponse struct {
	N       int         `json:"n"`
	Results interface{} `json:"results"`
}

type getAdminStatsResponse struct {
	Guilds     []*guildResponse     `json:"guilds"`
	VoiceConns []*voiceConnResponse `json:"voice_connections"`
	System     *systemStatsResponse `json:"system"`
}

type guildResponse struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type voiceConnResponse struct {
	Guild *guildResponse `json:"guild"`
	VCID  string         `json:"vc_id"`
}

type systemStatsResponse struct {
	Arch       string  `json:"arch"`
	OS         string  `json:"os"`
	GoVersion  string  `json:"go_version"`
	NumCPUs    int     `json:"cpu_used_cores"`
	GoRoutines int     `json:"go_routines"`
	HeapUse    uint64  `json:"heap_use_b"`
	StackUse   uint64  `json:"stack_use_b"`
	Uptime     float64 `json:"uptime_seconds"`
}

// -----------------------------------------------
// --- REST API HANDLERS

// GET /token
func (api *API) restGetTokenHandler(w http.ResponseWriter, r *http.Request, userID string) {
	if !checkMethodWithResponse(w, r, "GET") {
		return
	}

	if ok, _ := api.checkLimitWithResponse(w, r.RemoteAddr); !ok {
		return
	}

	guilds := discordbot.GetUsersGuilds(api.session, userID)
	if guilds == nil {
		errResponse(w, http.StatusForbidden, "you must be a member of a guild the bot is also member of")
		return
	}

	token, expire, err := api.auth.CreateToken(userID)
	if err != nil {
		errResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, &getTokenResponse{
		Token:  token,
		UserID: userID,
		Expire: expire,
	})
}

// GET /api/localsounds
func (api *API) restGetLocalSounds(w http.ResponseWriter, r *http.Request) {
	if !checkMethodWithResponse(w, r, "GET") {
		return
	}

	ok, userID := api.checkAuthWithResponse(w, r)
	if !ok {
		return
	}

	if ok, _ := api.checkLimitWithResponse(w, userID); !ok {
		return
	}

	queries := r.URL.Query()

	sort := queries.Get("sort")

	_, from, err := GetURLQueryInt(queries, "from", 0)
	if err != nil {
		errResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	okLimit, limit, err := GetURLQueryInt(queries, "limit", 1)
	if err != nil {
		errResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	soundList, err := api.player.GetLocalFiles()
	if err != nil {
		errResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch strings.ToUpper(sort) {
	case "NAME":
		soundList.SortByName()
	case "DATE":
		soundList.SortByDate()
	}

	if okLimit {
		if limit > len(soundList)-from {
			limit = len(soundList) - from
		}
		soundList = soundList[from : from+limit]
	} else {
		soundList = soundList[from:]
	}

	jsonResponse(w, http.StatusOK, &listResponse{
		N:       len(soundList),
		Results: soundList,
	})
}

// GET /api/logs/:GUILDID
func (api *API) restGetLogs(w http.ResponseWriter, r *http.Request) {
	if !checkMethodWithResponse(w, r, "GET") {
		return
	}

	ok, userID := api.checkAuthWithResponse(w, r)
	if !ok {
		return
	}

	if ok, _ := api.checkLimitWithResponse(w, userID); !ok {
		return
	}

	gidInd := strings.LastIndex(r.URL.Path, "/")
	if gidInd == -1 || gidInd == len(r.URL.Path)-1 {
		errResponse(w, http.StatusBadRequest, "GUILDID must be a valid snowflake value")
		return
	}

	guildID := r.URL.Path[gidInd+1:]

	queries := r.URL.Query()

	_, from, err := GetURLQueryInt(queries, "from", 0)
	if err != nil {
		errResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	okLimit, limit, err := GetURLQueryInt(queries, "limit", 1)
	if err != nil {
		errResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if !okLimit {
		limit = 1000
	}

	if _, err = api.session.GuildMember(guildID, userID); err != nil {
		errResponse(w, http.StatusForbidden, "you must be a member of this guild")
		return
	}

	logs, err := api.db.GetLogEntries(guildID, from, limit)
	if err != nil {
		errResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, &listResponse{
		N:       len(logs),
		Results: logs,
	})
}

// GET /api/stats/:GUILDID
func (api *API) restGetStats(w http.ResponseWriter, r *http.Request) {
	if !checkMethodWithResponse(w, r, "GET") {
		return
	}

	ok, userID := api.checkAuthWithResponse(w, r)
	if !ok {
		return
	}

	if ok, _ := api.checkLimitWithResponse(w, userID); !ok {
		return
	}

	gidInd := strings.LastIndex(r.URL.Path, "/")
	if gidInd == -1 || gidInd == len(r.URL.Path)-1 {
		errResponse(w, http.StatusBadRequest, "GUILDID must be a valid snowflake value")
		return
	}

	guildID := r.URL.Path[gidInd+1:]

	queries := r.URL.Query()

	okLimit, limit, err := GetURLQueryInt(queries, "limit", 1)
	if err != nil {
		errResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	if !okLimit {
		limit = 1000
	}

	if _, err = api.session.GuildMember(guildID, userID); err != nil {
		errResponse(w, http.StatusForbidden, "you must be a member of this guild")
		return
	}

	stats, err := api.db.GetSoundStats(guildID, limit)
	if err != nil {
		errResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, &listResponse{
		N:       len(stats),
		Results: stats,
	})
}

// GET /api/admin/stats
func (api *API) restGetAdminStats(w http.ResponseWriter, r *http.Request) {
	if !checkMethodWithResponse(w, r, "GET") {
		return
	}

	ok, userID := api.checkAuthWithResponse(w, r)
	if !ok {
		return
	}

	if !api.isAdmin(userID) {
		errResponse(w, http.StatusUnauthorized, "")
		return
	}

	status := new(getAdminStatsResponse)

	status.Guilds = make([]*guildResponse, len(api.session.State.Guilds))
	status.VoiceConns = make([]*voiceConnResponse, 0)

	for i, g := range api.session.State.Guilds {
		status.Guilds[i] = &guildResponse{
			ID:   g.ID,
			Name: g.Name,
		}

		for _, vs := range g.VoiceStates {
			if vs.UserID != api.session.State.User.ID {
				continue
			}

			status.VoiceConns = append(status.VoiceConns, &voiceConnResponse{
				Guild: &guildResponse{
					ID:   g.ID,
					Name: g.Name,
				},
				VCID: vs.ChannelID,
			})
		}
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	status.System = &systemStatsResponse{
		Uptime:     time.Since(static.Uptime).Seconds(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		NumCPUs:    runtime.NumCPU(),
		GoVersion:  runtime.Version(),
		GoRoutines: runtime.NumGoroutine(),
		StackUse:   memStats.StackInuse,
		HeapUse:    memStats.HeapInuse,
	}

	jsonResponse(w, 200, status)
}

// POST /api/admin/restart
func (api *API) restPostAdminRestart(w http.ResponseWriter, r *http.Request) {
	if !checkMethodWithResponse(w, r, "POST") {
		return
	}

	ok, userID := api.checkAuthWithResponse(w, r)
	if !ok {
		return
	}

	if !api.isAdmin(userID) {
		errResponse(w, http.StatusUnauthorized, "")
		return
	}

	jsonResponse(w, http.StatusOK, nil)
	go func() {
		api.teardownChan <- static.SigRestart
	}()
}

// POST /api/admin/refetch
func (api *API) restPostAdminRefetch(w http.ResponseWriter, r *http.Request) {
	if !checkMethodWithResponse(w, r, "POST") {
		return
	}

	ok, userID := api.checkAuthWithResponse(w, r)
	if !ok {
		return
	}

	if !api.isAdmin(userID) {
		errResponse(w, http.StatusUnauthorized, "")
		return
	}

	if err := api.player.FetchLocalSounds(); err != nil {
		errResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, nil)
}

// -----------------------------------------------
// --- FE HANDLERS

func (api *API) successfullAuthHandler(w http.ResponseWriter, r *http.Request, userID string) {
	guilds := discordbot.GetUsersGuilds(api.session, userID)
	if guilds == nil {
		errPageResponse(w, r, http.StatusForbidden, "")
		return
	}

	token, _, err := api.auth.CreateToken(userID)
	if err != nil {
		errPageResponse(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Add("Set-Cookie",
		fmt.Sprintf("token=%s; Max-Age=2147483647; Path=/", token))
	w.Header().Add("Set-Cookie",
		fmt.Sprintf("userid=%s; Max-Age=2147483647; Path=/", userID))
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (api *API) indexPageHandler(w http.ResponseWriter, r *http.Request) {
	ok, userID, err := api.checkAuthCookie(r)
	if err != nil {
		logger.Error("API :: checkAuthCookie: %s", err.Error())
		errPageResponse(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	if !ok || userID == "" {
		w.Header().Set("Location", "/login")
		w.WriteHeader(http.StatusTemporaryRedirect)
		return
	}

	guilds := discordbot.GetUsersGuilds(api.session, userID)
	if guilds == nil {
		errPageResponse(w, r, http.StatusForbidden, "")
		return
	}

	http.ServeFile(w, r, "./web/pages/index.html")
}

func (api *API) logoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Set-Cookie", "token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/;")
	w.Header().Add("Set-Cookie", "userid=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/;")
	http.ServeFile(w, r, "./web/pages/logout.html")
}

func (api *API) wsUpgradeHandler(w http.ResponseWriter, r *http.Request) {
	_, err := api.ws.NewConn(w, r, nil)
	if err != nil {
		logger.Error("API :: wsUpgradeHandler: %s", err.Error())
		errResponse(w, http.StatusBadRequest, err.Error())
		return
	}
}
