package gdetect

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func compareClients(c1 *Client, c2 *Client) (equal bool) {
	equal = c1.Endpoint == c2.Endpoint && c1.Token == c2.Token
	return
}

var token = "abcdef01-23456789-abcdef01-23456789-abcdef01"

func TestNewClient(t *testing.T) {
	type args struct {
		endpoint   string
		token      string
		insecure   bool
		httpClient *http.Client
	}
	tests := []struct {
		name       string
		args       args
		wantClient *Client
		wantErr    bool
	}{
		{
			name: "valid",
			args: args{
				endpoint:   "http://glimps/detect",
				token:      token,
				insecure:   false,
				httpClient: nil,
			},
			wantErr: false,
			wantClient: &Client{
				Endpoint: "http://glimps/detect",
				Token:    token,
			},
		},
		{
			name: "empty token",
			args: args{
				endpoint:   "http://glimps/detect",
				token:      "",
				insecure:   false,
				httpClient: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid char in token",
			args: args{
				endpoint:   "http://glimps/detect",
				token:      "tbcdef01-23456789-abcdef01-23456789-abcdef01",
				insecure:   false,
				httpClient: nil,
			},
			wantErr: true,
		},
		{
			name: "too little token",
			args: args{
				endpoint:   "http://glimps/detect",
				token:      "abcdef01",
				insecure:   false,
				httpClient: nil,
			},
			wantErr: true,
		},
		{
			name: "valid default http client",
			args: args{
				endpoint:   "http://glimps/detect",
				token:      token,
				insecure:   false,
				httpClient: http.DefaultClient,
			},
			wantErr: false,
			wantClient: &Client{
				Endpoint:   "http://glimps/detect",
				Token:      token,
				HttpClient: http.DefaultClient,
			},
		},
		{
			name: "valid custom http client",
			args: args{
				endpoint:   "http://glimps/detect",
				token:      token,
				insecure:   false,
				httpClient: &http.Client{Timeout: 2 * time.Second},
			},
			wantErr: false,
			wantClient: &Client{
				Endpoint:   "http://glimps/detect",
				Token:      token,
				HttpClient: &http.Client{Timeout: 2 * time.Second},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClient, err := NewClient(tt.args.endpoint, tt.args.token, tt.args.insecure, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantClient != nil {
				if !compareClients(gotClient, tt.wantClient) {
					t.Errorf("NewClient() = %v, want %v", gotClient, tt.wantClient)
				}
			}
		})
	}
}

func TestClient_SubmitFile(t *testing.T) {
	type args struct {
		ctx         context.Context
		filepath    string
		tags        []string
		description string
		bypassCache bool
		filename    string
	}

	filepath := "../../tests/samples/false_mirai"

	tests := []struct {
		name     string
		args     args
		timeout  time.Duration
		wantUuid string
		wantErr  bool
	}{
		{
			name: "VALID",
			args: args{
				ctx:         context.Background(),
				filepath:    filepath,
				description: "valid test",
			},
			wantErr:  false,
			wantUuid: "1234",
		},
		{
			name: "VALID WITH FILENAME",
			args: args{
				ctx:         context.Background(),
				filepath:    filepath,
				description: "valid test",
				filename:    "test.exe",
			},
			wantErr:  false,
			wantUuid: "1234",
		},
		{
			name: "INVALID FILE",
			args: args{
				ctx:      context.Background(),
				filepath: "not/a/file",
			},
			wantErr: true,
		},
		{
			name: "BAD REQUEST",
			args: args{
				ctx:         context.Background(),
				filepath:    filepath,
				description: "invalid file",
			},
			wantErr: true,
		},
		{
			name: "PARAMS USE",
			args: args{
				ctx:         context.Background(),
				filepath:    filepath,
				description: "file params",
				tags:        []string{"tag1", "tag2"},
				bypassCache: true,
			},
			wantErr:  false,
			wantUuid: "12345",
		},
		{
			name: "SUBMISSION STATUS FALSE",
			args: args{
				ctx:         context.Background(),
				filepath:    filepath,
				description: "submission status false",
			},
			wantErr: true,
		},
		{
			name: "BAD JSON RESPONSE",
			args: args{
				ctx:         context.Background(),
				filepath:    filepath,
				description: "bad json",
			},
			wantErr:  true,
			wantUuid: "",
		},
		{
			name: "TIMEOUT",
			args: args{
				ctx:         context.Background(),
				filepath:    filepath,
				description: "timeout",
			},
			wantErr: true,
			timeout: time.Millisecond * 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					if req.Header.Get("X-Auth-Token") != token {
						t.Errorf("handler.SubmitFile() %v error = unexpected TOKEN: %v", tt.name, req.Header.Get("X-Auth-Token"))
					}
					if req.Method != "POST" {
						t.Errorf("handler.SubmitFile() %v error = unexpected METHOD: %v", tt.name, req.Method)
					}
					if strings.TrimSpace(req.URL.Path) != "/api/lite/v2/submit" {
						t.Errorf("handler.SubmitFile() %v error = unexpected URL: %v", tt.name, strings.TrimSpace(req.URL.Path))
					}
					switch strings.TrimSpace(req.FormValue("description")) {
					case "valid test":
						rw.Write([]byte(`{"uuid":"1234", "status": true}`))
					case "invalid file":
						rw.WriteHeader(http.StatusBadRequest)
						rw.Write([]byte(`{"uuid":"1234", "status": true, "done": true}`))
					case "submission status false":
						rw.Write([]byte(`{"uuid":"1234", "status": false}`))
					case "bad json":
						rw.Write([]byte(`{"uuid":"1234", "status": false`))
					case "timeout":
						time.Sleep(time.Millisecond * 15)
						rw.Write([]byte(`{"uuid":"1234", "status": true}`))
					case "file params":
						if err := req.ParseMultipartForm(4096); err != nil {
							return
						}
						switch {
						case req.FormValue("bypass-cache") != "true", req.FormValue("description") != "file params", req.FormValue("tags") != "tag1,tag2":
							return
						}
						f, h, err := req.FormFile("file")

						switch {
						case err != nil:
							return
						case h.Filename != "false_mirai":
							return
						}
						buf := new(bytes.Buffer)
						if _, err := io.Copy(buf, f); err != nil {
							return
						}
						data := buf.String()
						if err != nil {
							return
						}
						if string(data) != "test content" {
							return
						}
						rw.Write([]byte(`{"status": true, "uuid": "12345"}`))
						return
					default:
						t.Errorf("handler.SubmitFile() %v error = unexpected file description: %v", tt.name, strings.TrimSpace(req.FormValue("description")))
					}
				}),
			)
			defer s.Close()

			client, err := NewClient(s.URL, token, false, nil)
			if err != nil {
				return
			}

			if tt.timeout != 0 {
				ctx, cancel := context.WithTimeout(tt.args.ctx, tt.timeout)
				defer cancel()
				tt.args.ctx = ctx
			}

			submitOptions := SubmitOptions{
				Description: tt.args.description,
				Tags:        tt.args.tags,
				BypassCache: tt.args.bypassCache,
				Filename:    tt.args.filename,
			}

			gotUuid, err := client.SubmitFile(tt.args.ctx, tt.args.filepath, submitOptions)

			if (err != nil) != tt.wantErr {
				t.Errorf("Client.SubmitFile() error = %v, wantErr = %t", err, tt.wantErr)
				return
			}
			if gotUuid != tt.wantUuid {
				t.Errorf("Client.SubmitFile() = %v, want %v", gotUuid, tt.wantUuid)
			}
		})
	}
}

func TestClient_GetResultByUUID(t *testing.T) {
	type args struct {
		ctx  context.Context
		uuid string
	}
	tests := []struct {
		name       string
		args       args
		wantResult Result
		wantErr    bool
		timeout    time.Duration
	}{
		{
			name: "VALID",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_valid_test",
			},
			wantErr:    false,
			wantResult: Result{UUID: "1234_valid_test", Done: true},
		},
		{
			name: "TIMEOUT",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_timeout",
			},
			wantErr: true,
			timeout: 5 * time.Millisecond,
		},
		{
			name: "NOT FOUND",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_not_found",
			},
			wantErr: true,
		},
		{
			name: "INTERNAL SERVER ERROR",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_server_error",
			},
			wantErr: true,
		},
		{
			name: "BAD JSON",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_bad_json",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					if req.Header.Get("X-Auth-Token") != token {
						t.Errorf("handler.GetResultByUUID() %v error = unexpected TOKEN: %v", tt.name, req.Header.Get("X-Auth-Token"))
					}
					if req.Method != "GET" {
						t.Errorf("handler.GetResultByUUID() %v error = unexpected METHOD: %v", tt.name, req.Method)
					}
					switch strings.TrimSpace(req.URL.Path) {
					case "/api/lite/v2/results/1234_valid_test":
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_valid_test", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_timeout":
						time.Sleep(15 * time.Millisecond)
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_not_found":
						rw.WriteHeader(http.StatusNotFound)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_server_error":
						rw.WriteHeader(http.StatusInternalServerError)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_bad_json":
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true "done": true`))
					default:
						t.Errorf("handler.GetResultByUUID() %v error = unexpected URL: %v", tt.name, strings.TrimSpace(req.URL.Path))
					}
				}),
			)
			defer s.Close()

			client, err := NewClient(s.URL, token, false, nil)
			if err != nil {
				return
			}

			if tt.timeout != 0 {
				ctx, cancel := context.WithTimeout(tt.args.ctx, tt.timeout)
				defer cancel()
				tt.args.ctx = ctx
			}

			gotResult, err := client.GetResultByUUID(tt.args.ctx, tt.args.uuid)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetResultByUUID() error = %v, wantErr = %t", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Client.GetResultByUUID() error = %v, wantErr = %t", err, tt.wantErr)
			}
		})
	}
}

func TestClient_GetResultBySHA256(t *testing.T) {
	type args struct {
		ctx    context.Context
		sha256 string
	}

	tests := []struct {
		name       string
		args       args
		wantResult Result
		wantErr    bool
		timeout    time.Duration
	}{
		{
			name: "VALID",
			args: args{
				ctx:    context.Background(),
				sha256: "1234_valid_test",
			},
			wantErr:    false,
			wantResult: Result{UUID: "1234_valid_test", Done: true},
		},
		{
			name: "NOT FOUND",
			args: args{
				ctx:    context.Background(),
				sha256: "1234_not_found",
			},
			wantErr: true,
		},
		{
			name: "FORBIDDEN",
			args: args{
				ctx:    context.Background(),
				sha256: "1234_forbidden",
			},
			wantErr: true,
		},
		{
			name: "INTERNAL SERVER ERROR",
			args: args{
				ctx:    context.Background(),
				sha256: "1234_server_error",
			},
			wantErr: true,
		},
		{
			name: "TIMEOUT",
			args: args{
				ctx:    context.Background(),
				sha256: "1234_timeout",
			},
			wantErr: true,
			timeout: time.Millisecond * 5,
		},
		{
			name: "BAD JSON",
			args: args{
				ctx:    context.Background(),
				sha256: "1234_bad_json",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					if req.Header.Get("X-Auth-Token") != token {
						t.Errorf("handler.GetResultBySHA256() %v error = unexpected TOKEN: %v", tt.name, req.Header.Get("X-Auth-Token"))
					}
					if req.Method != "GET" {
						t.Errorf("handler.GetResultBySHA256() %v error = unexpected METHOD: %v", tt.name, req.Method)
					}
					switch strings.TrimSpace(req.URL.Path) {
					case "/api/lite/v2/search/1234_valid_test":
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_valid_test", "status": true, "done": true}`))
					case "/api/lite/v2/search/1234_timeout":
						time.Sleep(15 * time.Millisecond)
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true, "done": true}`))
					case "/api/lite/v2/search/1234_not_found":
						rw.WriteHeader(http.StatusNotFound)
						rw.Write([]byte(`{"uuid":"1234_not_found", "status": true, "done": true}`))
					case "/api/lite/v2/search/1234_server_error":
						rw.WriteHeader(http.StatusInternalServerError)
						rw.Write([]byte(`{"uuid":"1234_server_error", "status": true, "done": true}`))
					case "/api/lite/v2/search/1234_bad_json":
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_bad_json", "status": true, "done": true`))
					case "/api/lite/v2/search/1234_forbidden":
						rw.WriteHeader(http.StatusForbidden)
						rw.Write([]byte(`{"uuid":"1234_forbidden", "status": true, "done": true}`))
					default:
						t.Errorf("handler.GetResultBySHA256() %v error = unexpected URL: %v", tt.name, strings.TrimSpace(req.URL.Path))
					}
				}),
			)
			defer s.Close()

			c, err := NewClient(s.URL, token, false, nil)
			if err != nil {
				return
			}

			if tt.timeout != 0 {
				ctx, cancel := context.WithTimeout(tt.args.ctx, tt.timeout)
				defer cancel()
				tt.args.ctx = ctx
			}
			gotResult, err := c.GetResultBySHA256(tt.args.ctx, tt.args.sha256)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetResultBySHA256() error = %v, wantErr = %t", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Client.GetUUIDResult() = %+v, want %+v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestClient_WaitForFile(t *testing.T) {
	type args struct {
		ctx         context.Context
		filepath    string
		tags        []string
		description string
		bypassCache bool
		timeout     time.Duration
		params      []int
		pullTime    time.Duration
	}
	tests := []struct {
		name       string
		args       args
		wantResult Result
		wantErr    bool
		timeout    time.Duration
	}{
		{
			name: "VALID",
			args: args{
				ctx:      context.Background(),
				filepath: "../../tests/samples/false_mirai",
				params:   []int{1},
				timeout:  180 * time.Second,
				pullTime: 15 * time.Millisecond,
			},
			wantResult: Result{UUID: "1234", Done: true},
			wantErr:    false,
		},
		{
			name: "TIMEOUT",
			args: args{
				ctx:      context.Background(),
				filepath: "../../tests/samples/false_cryptolocker",
				params:   []int{1},
				timeout:  time.Millisecond * 15,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					if req.Header.Get("X-Auth-Token") != token {
						t.Errorf("handler.WaitForFile() %v error = unexpected TOKEN: %v", tt.name, req.Header.Get("X-Auth-Token"))
					}
					switch strings.TrimSpace(req.URL.Path) {
					case "/api/lite/v2/submit":
						if req.Method != "POST" {
							t.Errorf("handler.WaitForFile() %v error = unexpected METHOD: %v", tt.name, req.Method)
						}
						if err := req.ParseMultipartForm(4096); err != nil {
							http.NotFoundHandler().ServeHTTP(rw, req)
							return
						}
						_, h, err := req.FormFile("file")
						if err != nil {
							return
						}
						switch h.Filename {
						case "false_mirai":
							rw.Write([]byte(`{"uuid":"1234", "status": true}`))
						case "false_cryptolocker":
							rw.Write([]byte(`{"uuid":"1234_never_done", "status": true}`))
						}
					case "/api/lite/v2/results/1234":
						if req.Method != "GET" {
							t.Errorf("handler.WaitForFile() %v error = unexpected METHOD: %v", tt.name, req.Method)
						}
						rw.Write([]byte(`{"uuid":"1234", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_never_done":
						if req.Method != "GET" {
							t.Errorf("handler.WaitForFile() %v error = unexpected METHOD: %v", tt.name, req.Method)
						}
						rw.Write([]byte(`{"uuid":"1234", "status": true, "done": false}`))
					default:
						t.Errorf("handler.WaitForFile() %v error = unexpected URL: %v", tt.name, strings.TrimSpace(req.URL.Path))
					}
				}),
			)
			defer s.Close()

			client, err := NewClient(s.URL, token, false, nil)
			if err != nil {
				return
			}
			if tt.timeout != 0 {
				ctx, cancel := context.WithTimeout(tt.args.ctx, tt.timeout)
				defer cancel()
				tt.args.ctx = ctx
			}

			waitForOptions := WaitForOptions{
				Tags:        tt.args.tags,
				Description: tt.args.description,
				BypassCache: tt.args.bypassCache,
				Timeout:     tt.args.timeout,
				PullTime:    tt.args.pullTime,
			}

			gotResult, err := client.WaitForFile(tt.args.ctx, tt.args.filepath, waitForOptions)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.WaitForFile() error = %v, wantErr = %t", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Client.WaitForFile() = %+v, want %+v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestClient_ExtractTokenViewURL(t *testing.T) {
	type fields struct {
		Endpoint string
		Token    string
		insecure bool
	}
	type args struct {
		result *Result
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantURLTokenView string
		wantErr          bool
	}{
		{
			name: "VALID",
			fields: fields{
				Endpoint: "http://gdetect/api",
				Token:    token,
				insecure: false,
			},
			args: args{
				result: &Result{
					UUID:  "1234",
					Token: "5678",
				},
			},
			wantErr:          false,
			wantURLTokenView: "http://gdetect/api/expert/en/analysis-redirect/5678",
		},
		{
			name: "NO TOKEN",
			fields: fields{
				Endpoint: "http://gdetect/api",
				Token:    token,
				insecure: false,
			},
			args: args{
				result: &Result{
					UUID: "1234",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				Endpoint: tt.fields.Endpoint,
				Token:    tt.fields.Token,
			}
			gotURLTokenView, err := c.ExtractTokenViewURL(tt.args.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.ExtractTokenViewURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotURLTokenView != tt.wantURLTokenView {
				t.Errorf("Client.ExtractTokenViewURL() = %v, want %v", gotURLTokenView, tt.wantURLTokenView)
			}
		})
	}
}

func TestClient_ExtractExpertViewURL(t *testing.T) {
	type fields struct {
		Endpoint string
		Token    string
		insecure bool
	}
	type args struct {
		result *Result
	}
	tests := []struct {
		name              string
		fields            fields
		args              args
		wantURLExpertView string
		wantErr           bool
	}{
		{
			name: "VALID",
			fields: fields{
				Endpoint: "http://gdetect/api",
				Token:    token,
				insecure: false,
			},
			args: args{
				result: &Result{
					UUID: "1234",
					SID:  "5678",
				},
			},
			wantErr:           false,
			wantURLExpertView: "http://gdetect/api/expert/en/analysis/advanced/5678",
		},
		{
			name: "NO TOKEN",
			fields: fields{
				Endpoint: "http://gdetect/api",
				Token:    token,
				insecure: false,
			},
			args: args{
				result: &Result{
					UUID: "1234",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				Endpoint: tt.fields.Endpoint,
				Token:    tt.fields.Token,
			}
			gotURLExpertView, err := c.ExtractExpertViewURL(tt.args.result)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.ExtractExpertViewURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotURLExpertView != tt.wantURLExpertView {
				t.Errorf("Client.ExtractExpertViewURL() = %v, want %v", gotURLExpertView, tt.wantURLExpertView)
			}
		})
	}
}

func Example_ClientSubmitFile() {
	// example mock up
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(`{"status":false,"error":"unauthorized"}`))
	}))

	defer srv.Close()

	client, err := NewClient(srv.URL, "2b886d5f-aa81d629-4299e60b-41b728ba-9bcbbc00", false, nil)
	if err != nil {
		fmt.Println(err)
	}

	result, err := client.SubmitFile(context.Background(), "/bin/sh", SubmitOptions{
		Tags:        []string{"test"},
		Description: "test submission",
		BypassCache: false,
	})
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(result)
	}
	// Output:
	// invalid response from endpoint, 401 Unauthorized: {"status":false,"error":"unauthorized"}
}

func TestClient_GetFullSubmissionByUUID(t *testing.T) {
	type args struct {
		ctx  context.Context
		uuid string
	}
	tests := []struct {
		name       string
		args       args
		wantResult interface{}
		wantErr    bool
		timeout    time.Duration
	}{
		{
			name: "TIMEOUT",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_timeout",
			},
			wantErr: true,
			timeout: 5 * time.Millisecond,
		},
		{
			name: "NOT FOUND",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_not_found",
			},
			wantErr: true,
		},
		{
			name: "INTERNAL SERVER ERROR",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_server_error",
			},
			wantErr: true,
		},
		{
			name: "BAD JSON",
			args: args{
				ctx:  context.Background(),
				uuid: "1234_bad_json",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := httptest.NewServer(
				http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
					if req.Header.Get("X-Auth-Token") != token {
						t.Errorf("handler.GetResultByUUID() %v error = unexpected TOKEN: %v", tt.name, req.Header.Get("X-Auth-Token"))
					}
					if req.Method != "GET" {
						t.Errorf("handler.GetResultByUUID() %v error = unexpected METHOD: %v", tt.name, req.Method)
					}
					switch strings.TrimSpace(req.URL.Path) {
					case "/api/lite/v2/results/1234_valid_test/full":
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_valid_test", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_timeout/full":
						time.Sleep(15 * time.Millisecond)
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_not_found/full":
						rw.WriteHeader(http.StatusNotFound)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_server_error/full":
						rw.WriteHeader(http.StatusInternalServerError)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true, "done": true}`))
					case "/api/lite/v2/results/1234_bad_json/full":
						rw.WriteHeader(http.StatusOK)
						rw.Write([]byte(`{"uuid":"1234_timeout", "status": true "done": true`))
					default:
						t.Errorf("handler.GetResultByUUID() %v error = unexpected URL: %v", tt.name, strings.TrimSpace(req.URL.Path))
					}
				}),
			)
			defer s.Close()

			client, err := NewClient(s.URL, token, false, nil)
			if err != nil {
				return
			}

			if tt.timeout != 0 {
				ctx, cancel := context.WithTimeout(tt.args.ctx, tt.timeout)
				defer cancel()
				tt.args.ctx = ctx
			}

			gotResult, err := client.GetFullSubmissionByUUID(tt.args.ctx, tt.args.uuid)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetFullSubmissionByUUID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("Client.GetFullSubmissionByUUID() = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
