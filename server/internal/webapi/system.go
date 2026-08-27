package webapi

import (
	"github.com/go-fuego/fuego"

	"github.com/bwees/mundus/server/internal/system"
)

type EnabledInput struct {
	Enabled bool `json:"enabled"`
}

type KeyInput struct {
	Key string `json:"key"`
}

func registerSystem(api *fuego.Server, d Deps) {
	fuego.Get(api, "/system/cloud", func(ctx fuego.ContextNoBody) (system.CloudStatus, error) {
		return d.System.CloudStatus(), nil
	}, fuego.OptionOperationID("getCloud"))

	fuego.Put(api, "/system/cloud", func(ctx fuego.ContextWithBody[EnabledInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		if err := d.System.SetCloudEnabled(body.Enabled); err != nil {
			return OK{}, err
		}
		return OK{OK: true}, nil
	}, fuego.OptionOperationID("setCloud"))

	fuego.Get(api, "/system/ssh", func(ctx fuego.ContextNoBody) (system.SSHStatus, error) {
		return d.System.SSHStatus(), nil
	}, fuego.OptionOperationID("getSsh"))

	fuego.Put(api, "/system/ssh", func(ctx fuego.ContextWithBody[EnabledInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		if err := d.System.SetSSHEnabled(body.Enabled); err != nil {
			return OK{}, err
		}
		return OK{OK: true}, nil
	}, fuego.OptionOperationID("setSsh"))

	fuego.Post(api, "/system/ssh/keys", func(ctx fuego.ContextWithBody[KeyInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		if err := d.System.AddKey(body.Key); err != nil {
			return OK{}, err
		}
		return OK{OK: true}, nil
	}, fuego.OptionOperationID("addSshKey"))

	fuego.Post(api, "/system/ssh/keys/delete", func(ctx fuego.ContextWithBody[KeyInput]) (OK, error) {
		body, err := ctx.Body()
		if err != nil {
			return OK{}, err
		}
		if err := d.System.RemoveKey(body.Key); err != nil {
			return OK{}, err
		}
		return OK{OK: true}, nil
	}, fuego.OptionOperationID("deleteSshKey"))
}
