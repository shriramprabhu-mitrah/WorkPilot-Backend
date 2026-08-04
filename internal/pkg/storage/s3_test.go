package storage

import "testing"

func TestDerivePublicEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "path style s3 endpoint",
			endpoint: "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/s3",
			want:     "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/object/public",
		},
		{
			name:     "embedded s3 path",
			endpoint: "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/s3/some",
			want:     "https://rywvrcvpgeenhvlyrtaj.storage.supabase.co/storage/v1/object/public/some",
		},
		{
			name:     "already public object endpoint",
			endpoint: "https://rywvrcvpgeenhvlyrtaj.supabase.co/storage/v1/object/public",
			want:     "https://rywvrcvpgeenhvlyrtaj.supabase.co/storage/v1/object/public",
		},
		{
			name:     "missing s3 segment",
			endpoint: "https://example.com/base",
			want:     "https://example.com/base/object/public",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := derivePublicEndpoint(tt.endpoint)
			if got != tt.want {
				t.Fatalf("derivePublicEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}
