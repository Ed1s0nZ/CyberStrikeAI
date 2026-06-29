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
