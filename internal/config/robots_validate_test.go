package config

import "testing"

func TestValidateWecomConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     RobotWecomConfig
		wantErr bool
	}{
		{
			name:    "disabled without token",
			cfg:     RobotWecomConfig{Enabled: false, Token: ""},
			wantErr: false,
		},
		{
			name:    "enabled with token",
			cfg:     RobotWecomConfig{Enabled: true, Token: "secret"},
			wantErr: false,
		},
		{
			name:    "enabled without token",
			cfg:     RobotWecomConfig{Enabled: true, Token: ""},
			wantErr: true,
		},
		{
			name:    "enabled with whitespace token",
			cfg:     RobotWecomConfig{Enabled: true, Token: "   "},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateWecomConfig(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateWecomConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRobotAuthorization(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RobotAuthorizationConfig
		wantErr bool
	}{
		{name: "default user binding", cfg: RobotAuthorizationConfig{}, wantErr: false},
		{name: "explicit user binding", cfg: RobotAuthorizationConfig{Mode: RobotAuthModeUserBinding}, wantErr: false},
		{name: "service account", cfg: RobotAuthorizationConfig{Mode: RobotAuthModeServiceAccount, ServiceUserID: "svc-1", AllowedExternalUsers: []string{"t:x|u:y"}}, wantErr: false},
		{name: "missing service user", cfg: RobotAuthorizationConfig{Mode: RobotAuthModeServiceAccount, AllowedExternalUsers: []string{"t:x|u:y"}}, wantErr: true},
		{name: "admin allowed with exact sender", cfg: RobotAuthorizationConfig{Mode: RobotAuthModeServiceAccount, ServiceUserID: "admin", AllowedExternalUsers: []string{"t:x|u:y"}}, wantErr: false},
		{name: "allowlist required", cfg: RobotAuthorizationConfig{Mode: RobotAuthModeServiceAccount, ServiceUserID: "svc-1"}, wantErr: true},
		{name: "wildcard forbidden", cfg: RobotAuthorizationConfig{Mode: RobotAuthModeServiceAccount, ServiceUserID: "svc-1", AllowedExternalUsers: []string{"*"}}, wantErr: true},
		{name: "unknown mode", cfg: RobotAuthorizationConfig{Mode: "open"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRobotAuthorization(tt.cfg, "robots.lark"); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateRobotAuthorization() error=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
