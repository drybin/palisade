package config

import (
    "errors"
    "time"
    
    "github.com/drybin/palisade/pkg/env"
    "github.com/drybin/palisade/pkg/wrap"
    validation "github.com/go-ozzo/ozzo-validation/v4"
)

type Config struct {
    ServiceName string
    PassPhrase  string
    TgConfig    TgConfig
    MexcConfig  MexcConfig
}

type TgConfig struct {
    BotToken string
    ChatId   string
    Timeout  time.Duration
}

type MexcConfig struct {
    ApiKey  string
    Secret  string
    BaseUrl string
}

func (c Config) Validate() error {
    var errs []error
    
    //err := validation.ValidateStruct(&c,
    //	validation.Field(&c.MexcConfig.ApiKey, validation.Required),
    //	validation.Field(&c.MexcConfig.Secret, validation.Required),
    //)
    
    err := c.MexcConfig.Validate()
    if err != nil {
        return wrap.Errorf("failed to validate cli config: %w", err)
    }
    
    return errors.Join(errs...)
}

func (c MexcConfig) Validate() error {
    var errs []error
    
    err := validation.ValidateStruct(&c,
        validation.Field(&c.ApiKey, validation.Required),
        validation.Field(&c.Secret, validation.Required),
        validation.Field(&c.BaseUrl, validation.Required),
    )
    if err != nil {
        return wrap.Errorf("failed to validate MexC config: %w", err)
    }
    
    return errors.Join(errs...)
}

func InitConfig() (*Config, error) {
    config := Config{
        ServiceName: env.GetString("APP_NAME", "fead-and-greed"),
        TgConfig:    initTgConfig(),
        MexcConfig:  initMexcConfig(),
    }
    
    if err := config.Validate(); err != nil {
        return nil, err
    }
    
    return &config, nil
}

func initTgConfig() TgConfig {
    return TgConfig{
        BotToken: env.GetString("TG_BOT_TOKEN", ""),
        ChatId:   env.GetString("TG_CHAT_ID", ""),
        Timeout:  10 * time.Second,
    }
}

func initMexcConfig() MexcConfig {
    return MexcConfig{
        ApiKey:  env.GetString("MEXC_API_KEY", ""),
        Secret:  env.GetString("MEXC_SECRET", ""),
        BaseUrl: env.GetString("MEXC_API_URL", ""),
    }
}
