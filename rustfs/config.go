package rustfs

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/braver-braver/lettuce"
)

const defaultRegion = "us-east-1"

// Config configures a RustFS S3 backend.
//
// Path-style addressing is used by default because it is RustFS's default S3
// access mode. Set VirtualHost only when the RustFS server and DNS are
// explicitly configured for virtual-host addressing.
type Config struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	SessionToken string
	VirtualHost  bool
	HTTPClient   *http.Client
}

// New constructs a RustFS-backed Store.
func New(ctx context.Context, cfg Config) (*Store, error) {
	if ctx == nil {
		return nil, configError("context must not be nil")
	}

	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			cfg.SessionToken,
		)),
	}
	if cfg.HTTPClient != nil {
		loadOptions = append(loadOptions, awsconfig.WithHTTPClient(cfg.HTTPClient))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, storageError("configure", "", err)
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = !cfg.VirtualHost
	})

	return newStore(cfg.Bucket, client, transfermanager.New(client)), nil
}

func normalizeConfig(cfg Config) (Config, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return Config{}, configError("endpoint is required")
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return Config{}, configError("invalid endpoint")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Config{}, configError("endpoint scheme must be http or https")
	}
	if u.Host == "" {
		return Config{}, configError("endpoint host is required")
	}
	if u.User != nil {
		return Config{}, configError("endpoint must not contain credentials")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Config{}, configError("endpoint must not contain query or fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return Config{}, configError("endpoint must not contain a path")
	}
	u.Path = ""
	u.RawPath = ""
	cfg.Endpoint = strings.TrimRight(u.String(), "/")

	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	if cfg.Bucket == "" {
		return Config{}, configError("bucket is required")
	}
	if strings.ContainsAny(cfg.Bucket, "/\\") {
		return Config{}, configError("bucket must not contain path separators")
	}

	if cfg.AccessKey == "" {
		return Config{}, configError("access key is required")
	}
	if cfg.SecretKey == "" {
		return Config{}, configError("secret key is required")
	}

	cfg.Region = strings.TrimSpace(cfg.Region)
	if cfg.Region == "" {
		cfg.Region = defaultRegion
	}

	return cfg, nil
}

func configError(message string) error {
	return &lettuce.Error{
		Provider: providerName,
		Op:       "configure",
		Err:      fmt.Errorf("%s: %w", message, lettuce.ErrInvalid),
	}
}
