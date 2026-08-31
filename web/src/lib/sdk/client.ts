/**
 * OpenAPI
 * 0.0.1
 * DO NOT MODIFY - This file has been generated using oazapfts.
 * See https://www.npmjs.com/package/oazapfts
 */
import * as Oazapfts from "@oazapfts/runtime";
import * as QS from "@oazapfts/runtime/query";
export const defaults: Oazapfts.Defaults<Oazapfts.CustomHeaders> = {
    headers: {},
    baseUrl: "/"
};
const oazapfts = Oazapfts.runtime(defaults);
export const servers = {};
export type CredentialsInput = {
    password: string;
};
export type TokenDto = {
    token: string;
};
export type ErrorItem = {
    /** Additional information about the error */
    more?: {
        [key: string]: any;
    } | {
        [key: string]: any;
    };
    /** For example, name of the parameter that caused the error */
    name: string;
    /** Human readable error message */
    reason: string;
};
export type HttpError = {
    /** Human readable error message */
    detail?: string;
    errors?: ErrorItem[] | null;
    instance?: string;
    /** HTTP status code */
    status?: number;
    /** Short title of the error */
    title?: string;
    /** URL of the error type. Can be used to lookup the error in a documentation */
    "type"?: string;
};
export type Ok = {
    ok: boolean;
};
export type ChangePasswordInput = {
    current_password: string;
    new_password: string;
};
export type SessionDto = {
    authenticated: boolean;
};
export type AuthStatusDto = {
    setup_required: boolean;
};
export type ModeDto = {
    fan_level: number;
    times: number;
    "type": string;
    water_level: number;
};
export type CleanRequest = {
    mode: ModeDto;
    rooms: string[] | null;
};
export type MqttConfigDto = {
    base_topic: string;
    broker: string;
    connected: boolean;
    discovery_prefix: string;
    has_password: boolean;
    username: string;
};
export type MqttConfigInput = {
    base_topic: string;
    broker: string;
    discovery_prefix: string;
    password: string;
    username: string;
};
export type RoomGeomDto = {
    color_type: number;
    geometry: number[] | null;
    graph: number[] | null;
    id: string;
    name: string;
};
export type ZoneDto = {
    geometry: number[] | null;
    id: string;
    kind: string;
    name: string;
};
export type MapDto = {
    height: number;
    image_png: string;
    origin_x: number;
    origin_y: number;
    resolution: number;
    rooms: RoomGeomDto[] | null;
    uuid: string;
    width: number;
    zones: ZoneDto[] | null;
};
export type MergeRoomsInput = {
    ids: string[] | null;
    name: string;
};
export type RenameRoomInput = {
    id: string;
    name: string;
};
export type SplitRoomInput = {
    id: string;
    line: any;
    new_name: string;
};
export type TrackDto = {
    points: number[] | null;
};
export type AddZoneInput = {
    geometry: number[] | null;
    kind: string;
    name: string;
};
export type ZoneIdInput = {
    id: string;
};
export type UpdateZoneInput = {
    geometry: number[] | null;
    id: string;
    name: string;
};
export type RoomDto = {
    id: string;
    name: string;
};
export type SelfCleanRequest = {
    action: number;
};
export type Option = {
    label: string;
    value: number;
};
export type Setting = {
    "default": number;
    icon: string;
    key: string;
    kind: string;
    max?: number;
    min?: number;
    name: string;
    options?: Option[] | null;
    prop: number;
};
export type SettingsDto = {
    schema: Setting[] | null;
    values: {
        [key: string]: number;
    } | {
        [key: string]: number;
    };
};
export type SettingsInput = {
    values: {
        [key: string]: number;
    } | {
        [key: string]: number;
    };
};
export type StateDto = {
    battery_level: number;
    charging: boolean;
    cloud_connected: boolean;
    device_name: string;
    docked: boolean;
    error_code: number;
    fan_speed: string;
    run_state: string;
    state: string;
    working_status: number;
};
export type CloudStatus = {
    bound: boolean;
    connected: boolean;
    enabled: boolean;
};
export type EnabledInput = {
    enabled: boolean;
};
export type SshKey = {
    comment: string;
    key: string;
    "type": string;
};
export type SshStatus = {
    enabled: boolean;
    keys: SshKey[] | null;
};
export type KeyInput = {
    key: string;
};
export type Status = {
    current_version: string;
    error?: string;
    last_checked: string;
    latest_version: string;
    state: string;
    update_available: boolean;
};
export type UnknownInterface = any;
/**
 * func3
 */
export function login(credentialsInput: CredentialsInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: TokenDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/auth/login", oazapfts.json({
        ...opts,
        method: "POST",
        body: credentialsInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func4
 */
export function logout({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/auth/logout", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func6
 */
export function changePassword(changePasswordInput: ChangePasswordInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: TokenDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/auth/password", oazapfts.json({
        ...opts,
        method: "POST",
        body: changePasswordInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func5
 */
export function getSession({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: SessionDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/auth/session", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func2
 */
export function setupAuth(credentialsInput: CredentialsInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: TokenDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/auth/setup", oazapfts.json({
        ...opts,
        method: "POST",
        body: credentialsInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func1
 */
export function getAuthStatus({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: AuthStatusDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/auth/status", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func3
 */
export function startClean(cleanRequest: CleanRequest, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/clean", oazapfts.json({
        ...opts,
        method: "POST",
        body: cleanRequest,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func12
 */
export function getMqttConfig({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: MqttConfigDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/config/mqtt", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func13
 */
export function setMqttConfig(mqttConfigInput: MqttConfigInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: MqttConfigDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/config/mqtt", oazapfts.json({
        ...opts,
        method: "PUT",
        body: mqttConfigInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func1
 */
export function dock({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/dock", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func1
 */
export function locate({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/locate", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func1
 */
export function getMap({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: MapDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func4
 */
export function mergeRooms(mergeRoomsInput: MergeRoomsInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map/merge", oazapfts.json({
        ...opts,
        method: "POST",
        body: mergeRoomsInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func3
 */
export function renameRoom(renameRoomInput: RenameRoomInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map/room", oazapfts.json({
        ...opts,
        method: "PUT",
        body: renameRoomInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func5
 */
export function splitRoom(splitRoomInput: SplitRoomInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map/split", oazapfts.json({
        ...opts,
        method: "POST",
        body: splitRoomInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func2
 */
export function getMapTrack({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: TrackDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map/track", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func6
 */
export function addZone(addZoneInput: AddZoneInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map/zone", oazapfts.json({
        ...opts,
        method: "POST",
        body: addZoneInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func8
 */
export function deleteZone(zoneIdInput: ZoneIdInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map/zone/delete", oazapfts.json({
        ...opts,
        method: "POST",
        body: zoneIdInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func7
 */
export function updateZone(updateZoneInput: UpdateZoneInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/map/zone/update", oazapfts.json({
        ...opts,
        method: "POST",
        body: updateZoneInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func1
 */
export function pause({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/pause", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func1
 */
export function resume({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/resume", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func2
 */
export function getRooms({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: RoomDto[];
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/rooms", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func9
 */
export function selfClean(selfCleanRequest: SelfCleanRequest, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/self-clean", oazapfts.json({
        ...opts,
        method: "POST",
        body: selfCleanRequest,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func10
 */
export function getSettings({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: SettingsDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/settings", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func11
 */
export function setSettings(settingsInput: SettingsInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/settings", oazapfts.json({
        ...opts,
        method: "PUT",
        body: settingsInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func1
 */
export function getState({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: StateDto;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/state", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func1
 */
export function stop({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/stop", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func1
 */
export function getCloud({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: CloudStatus;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/system/cloud", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func2
 */
export function setCloud(enabledInput: EnabledInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/system/cloud", oazapfts.json({
        ...opts,
        method: "PUT",
        body: enabledInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func3
 */
export function getSsh({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: SshStatus;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/system/ssh", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func4
 */
export function setSsh(enabledInput: EnabledInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/system/ssh", oazapfts.json({
        ...opts,
        method: "PUT",
        body: enabledInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func5
 */
export function addSshKey(keyInput: KeyInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/system/ssh/keys", oazapfts.json({
        ...opts,
        method: "POST",
        body: keyInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func6
 */
export function deleteSshKey(keyInput: KeyInput, { accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/system/ssh/keys/delete", oazapfts.json({
        ...opts,
        method: "POST",
        body: keyInput,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    })));
}
/**
 * func1
 */
export function getUpdate({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Status;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/update", {
        ...opts,
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func3
 */
export function applyUpdate({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Ok;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/update/apply", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
/**
 * func2
 */
export function checkUpdate({ accept }: {
    accept?: string;
} = {}, opts?: Oazapfts.RequestOpts) {
    return oazapfts.ok(oazapfts.fetchJson<{
        status: 200;
        data: Status;
    } | {
        status: 400;
        data: HttpError;
    } | {
        status: 500;
        data: HttpError;
    }>("/api/update/check", {
        ...opts,
        method: "POST",
        headers: oazapfts.mergeHeaders(opts?.headers, {
            Accept: accept
        })
    }));
}
