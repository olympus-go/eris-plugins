package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/mitchellh/mapstructure"
	"github.com/olympus-go/eris/utils"
	"golang.org/x/exp/slices"
)

var ErrFieldNotExist = errors.New("field does not exist")

func (p *Plugin) configHandler(discordSession *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		command := i.ApplicationCommandData()
		if command.Name != "config" || len(command.Options) == 0 || len(command.Options[0].Options) == 0 {
			return
		}

		if i.Interaction.GuildID == "" {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("I can't do that in a DM, sry.").
				SendWithLog(p.logger)
			return
		}

		pluginOption := utils.GetCommandOption(i.ApplicationCommandData(), "config", command.Options[0].Name)
		actionOption := utils.GetCommandOption(*pluginOption, command.Options[0].Name, command.Options[0].Options[0].Name)

		config, ok := p.configs.Get(pluginOption.Name)
		if !ok {
			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message(fmt.Sprintf("No config found for %s.", command.Options[0].Name)).
				SendWithLog(p.logger)
			return
		}

		switch actionOption.Name {
		case "get":
			b, err := json.MarshalIndent(config.config, "", "  ")
			if err != nil {
				p.logger.Error("failed to marshal config as json", slog.String("error", err.Error()))
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("Something went wrong.").
					SendWithLog(p.logger)
				return
			}

			keyOption := utils.GetCommandOption(*actionOption, "get", "key")
			if keyOption != nil {
				key := keyOption.StringValue()

				v, err := getConfig(config.config, key)
				if err != nil && errors.Is(err, ErrFieldNotExist) {
					utils.InteractionResponse(discordSession, i.Interaction).
						Ephemeral().
						Message(fmt.Sprintf("I don't know what \"%s\" is.", key)).
						SendWithLog(p.logger)
					return
				} else if err != nil {
					utils.InteractionResponse(discordSession, i.Interaction).
						Ephemeral().
						Message("Something went wrong.").
						SendWithLog(p.logger)
					return
				}

				var valueString string
				switch v.(type) {
				case string:
					valueString = v.(string)
				case []string, *[]string:
					valueString += "["
					for _, s := range v.([]string) {
						valueString += s + ", "
					}

					valueString = strings.TrimSuffix(valueString, ", ") + "]"
				default:
					p.logger.Error("failed to parse key")
					utils.InteractionResponse(discordSession, i.Interaction).
						Ephemeral().
						Message("Something went wrong.").
						SendWithLog(p.logger)
				}

				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message(fmt.Sprintf("```%s: %s```", key, valueString)).
					SendWithLog(p.logger)

				return
			}

			message := string(b)

			if len(message) < 2000 {
				message = fmt.Sprintf("%s config:\n```\n", pluginOption.Name) + message + "```\n"

				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message(message).
					SendWithLog(p.logger)
			} else {
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("The config is too big to write here. I have to attach it as a file.").
					SendWithLog(p.logger)

				filename := fmt.Sprintf("%s_config.yaml", pluginOption.Name)
				discordSession.ChannelFileSend(i.Interaction.ChannelID, filename, strings.NewReader(message))
			}
		case "set":
			if len(p.adminIds) > 0 && !slices.Contains(p.adminIds, utils.GetInteractionUserId(i.Interaction)) {
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("You don't have permissions to change configs.").
					SendWithLog(p.logger)
				return
			}

			keyOption := utils.GetCommandOption(*actionOption, "set", "key")
			if keyOption == nil {
				p.logger.Error("failed to parse key")
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("Something went wrong.").
					SendWithLog(p.logger)
				return
			}

			valueOption := utils.GetCommandOption(*actionOption, "set", "value")
			if valueOption == nil {
				p.logger.Error("failed to parse value")

				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("Something went wrong.").
					SendWithLog(p.logger)
				return
			}

			truncateOption := utils.GetCommandOption(*actionOption, "set", "truncate")
			truncate := false
			if truncateOption != nil {
				truncate = truncateOption.BoolValue()
			}

			key := keyOption.StringValue()
			value := valueOption.StringValue()

			// Shortcut to empty a string. RIP if you wanted to set a field to literally '""'.
			if value == "\"\"" {
				value = ""
			}

			if err := setConfig(config.config, key, value, truncate); err != nil && errors.Is(err, ErrFieldNotExist) {
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message(fmt.Sprintf("I don't know what \"%s\" is.", key)).
					SendWithLog(p.logger)
				return
			} else if err != nil {
				p.logger.Error("failed to set config", slog.String("error", err.Error()))
				utils.InteractionResponse(discordSession, i.Interaction).
					Ephemeral().
					Message("Something went wrong.").
					SendWithLog(p.logger)
				return
			}

			utils.InteractionResponse(discordSession, i.Interaction).
				Ephemeral().
				Message("Updated.").
				SendWithLog(p.logger)

			config.fn()
		}
	}
}

func getConfig(t any, key string) (any, error) {
	v := make(map[string]any)

	if err := mapstructure.Decode(t, &v); err != nil {
		return nil, err
	}

	var fields []string
	if strings.Contains(key, ".") {
		fields = strings.Split(strings.TrimSpace(key), ".")
	} else {
		fields = []string{key}
	}

	currentValue := v
	var ok bool
	for i, field := range fields {
		field = strings.TrimSpace(field)

		if _, ok = currentValue[field]; !ok {
			return nil, ErrFieldNotExist
		}

		// Set the value when at the end of the split list, otherwise keep traversing the map.
		if i == len(fields)-1 {
			return currentValue[field], nil
		} else {
			currentValue, ok = currentValue[field].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("failed to decode config")
			}
		}
	}

	// This should never happen
	return nil, ErrFieldNotExist
}

func setConfig(t any, key string, value string, truncate bool) error {
	v := make(map[string]any)

	if err := mapstructure.Decode(t, &v); err != nil {
		return err
	}

	var fields []string
	if strings.Contains(key, ".") {
		fields = strings.Split(strings.TrimSpace(key), ".")
	} else {
		fields = []string{key}
	}

	currentValue := v
	var ok bool
	for i, field := range fields {
		field = strings.TrimSpace(field)

		if _, ok = currentValue[field]; !ok {
			return ErrFieldNotExist
		}

		// Set the value when at the end of the split list, otherwise keep traversing the map.
		if i == len(fields)-1 {
			switch currentValue[field].(type) {
			case string:
				currentValue[field] = value
			case []string:
				if len(value) == 0 {
					if truncate {
						currentValue[field] = []string{}
					}
					break
				}

				values := strings.Split(value, ",")
				if len(values) < 1 {
					return fmt.Errorf("string slices should be delimited with a comma")
				}

				var prunedValues []string
				for vIndex := range values {
					trimmedValue := strings.TrimSpace(values[vIndex])
					if len(trimmedValue) > 0 {
						prunedValues = append(prunedValues, trimmedValue)
					}
				}

				if truncate {
					currentValue[field] = prunedValues
				} else {
					currentValues, _ := currentValue[field].([]string)
					currentValue[field] = append(currentValues, prunedValues...)
				}
			case *[]string:
				if len(value) == 0 {
					if truncate {
						currentValue[field] = &[]string{}
					}
					break
				}

				values := strings.Split(value, ",")
				if len(values) < 1 {
					return fmt.Errorf("string slices should be delimited with a comma")
				}

				var prunedValues []string
				for vIndex := range values {
					trimmedValue := strings.TrimSpace(values[vIndex])
					if len(trimmedValue) > 0 {
						prunedValues = append(prunedValues, trimmedValue)
					}
				}

				if truncate {
					currentValue[field] = &prunedValues
				} else {
					currentValues, _ := currentValue[field].(*[]string)
					appendedValues := append(*currentValues, prunedValues...)
					currentValue[field] = &appendedValues
				}
			default:
				return fmt.Errorf("failed to decode config: config field must be a string")
			}
		} else {
			currentValue, ok = currentValue[field].(map[string]any)
			if !ok {
				return fmt.Errorf("failed to decode config")
			}
		}
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{ZeroFields: true, Result: t})
	if err != nil {
		return err
	}

	return decoder.Decode(v)
}
