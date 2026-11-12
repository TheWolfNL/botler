import {SettingsService} from "../bindings/github.com/TheWolfNL/botler";
import type {AgentInfo} from "../bindings/github.com/TheWolfNL/botler";
import type {Config, KindSettings} from "../bindings/github.com/TheWolfNL/botler/internal/config";

const kinds = [
    {key: "done", name: "Done"},
    {key: "failed", name: "Failed"},
    {key: "input", name: "Needs input"},
] as const;
type Kind = (typeof kinds)[number]["key"];

const defaultSounds: Record<Kind, string> = {done: "Glass", failed: "Basso", input: "Ping"};

export function initSettings() {
    const form = document.getElementById("settings") as HTMLFormElement;
    const status = document.getElementById("status")!;
    const template = document.getElementById("kind-template") as HTMLTemplateElement;
    const kindsCard = document.getElementById("kinds")!;
    let statusTimer: ReturnType<typeof setTimeout>;
    let config: Config;

    function flash(message: string) {
        status.textContent = message;
        clearTimeout(statusTimer);
        statusTimer = setTimeout(() => (status.textContent = ""), 2500);
    }

    function named<T extends HTMLElement>(name: string): T {
        return form.elements.namedItem(name) as unknown as T;
    }

    const fieldsets = new Map<Kind, HTMLFieldSetElement>();
    for (const {key, name} of kinds) {
        const fs = (template.content.cloneNode(true) as DocumentFragment).querySelector("fieldset")!;
        fs.dataset.kind = key;
        fs.querySelector(".dot")!.classList.add(key);
        fs.querySelector(".kind-name")!.textContent = name;
        kindsCard.append(fs);
        fieldsets.set(key, fs);
    }

    function field<T extends HTMLElement>(kind: Kind, name: string): T {
        return fieldsets.get(kind)!.querySelector(`[data-field="${name}"]`) as T;
    }

    function readKind(kind: Kind): KindSettings {
        return {
            enabled: field<HTMLInputElement>(kind, "enabled").checked,
            banner: field<HTMLInputElement>(kind, "banner").checked,
            autoHide: Number(field<HTMLSelectElement>(kind, "autoHide").value),
            sound: field<HTMLInputElement>(kind, "sound").checked,
            soundName: field<HTMLSelectElement>(kind, "soundName").value,
        };
    }

    function read(): Config {
        return {
            ...config,
            done: readKind("done"),
            failed: readKind("failed"),
            input: readKind("input"),
            bannerPosition: named<HTMLSelectElement>("bannerPosition").value,
            onlyWhenAway: named<HTMLInputElement>("onlyWhenAway").checked,
            trayFlash: named<HTMLInputElement>("trayFlash").checked,
            trayMode: named<HTMLSelectElement>("trayMode").value,
        };
    }

    function group(label: string, names: string[] | null) {
        const g = document.createElement("optgroup");
        g.label = label;
        g.append(...(names ?? []).map((name) => new Option(name, name)));
        return g;
    }

    async function loadSounds() {
        const sounds = await SettingsService.Sounds();
        for (const {key} of kinds) {
            const select = field<HTMLSelectElement>(key, "soundName");
            const current = select.value;
            const groups = [group("Custom", sounds.custom), group("System", sounds.system)];
            select.replaceChildren(...groups.filter((g) => g.children.length > 0));
            if (current) select.value = current;
        }
    }

    function updateEnabled() {
        for (const {key} of kinds) {
            const fs = fieldsets.get(key)!;
            const enabled = field<HTMLInputElement>(key, "enabled").checked;
            const banner = enabled && field<HTMLInputElement>(key, "banner").checked;
            const sound = enabled && field<HTMLInputElement>(key, "sound").checked;
            const soundName = field<HTMLSelectElement>(key, "soundName").value;
            field<HTMLInputElement>(key, "banner").disabled = !enabled;
            field<HTMLInputElement>(key, "sound").disabled = !enabled;
            field<HTMLSelectElement>(key, "autoHide").disabled = !banner;
            field<HTMLSelectElement>(key, "soundName").disabled = !sound;
            fs.querySelector<HTMLButtonElement>(".remove-sound")!.disabled = !sound || !soundName.includes(".");
            fs.querySelector<HTMLButtonElement>(".preview")!.disabled = !sound;
            fs.querySelector<HTMLButtonElement>(".add-sound")!.disabled = !sound;
            fs.querySelector<HTMLButtonElement>(".test")!.disabled = !banner && !sound;
            fs.classList.toggle("off", !enabled);
        }
        named<HTMLInputElement>("trayFlash").disabled = named<HTMLSelectElement>("trayMode").value === "never";
    }

    async function load() {
        const [cfg, login] = await Promise.all([SettingsService.Get(), SettingsService.LaunchAtLogin(), loadSounds()]);
        config = cfg;
        for (const {key} of kinds) {
            const ks = cfg[key];
            field<HTMLInputElement>(key, "enabled").checked = ks.enabled;
            field<HTMLInputElement>(key, "banner").checked = ks.banner;
            field<HTMLSelectElement>(key, "autoHide").value = String(ks.autoHide);
            field<HTMLInputElement>(key, "sound").checked = ks.sound;
            field<HTMLSelectElement>(key, "soundName").value = ks.soundName;
        }
        named<HTMLSelectElement>("bannerPosition").value = cfg.bannerPosition;
        named<HTMLInputElement>("onlyWhenAway").checked = cfg.onlyWhenAway;
        named<HTMLInputElement>("trayFlash").checked = cfg.trayFlash;
        named<HTMLSelectElement>("trayMode").value = cfg.trayMode;
        loginToggle.checked = login;
        updateEnabled();
    }

    async function save() {
        updateEnabled();
        try {
            config = read();
            await SettingsService.Save(config);
            flash("Saved");
        } catch (err) {
            flash(`Save failed: ${err}`);
        }
    }

    const loginToggle = named<HTMLInputElement>("launchAtLogin");

    form.addEventListener("change", async (event) => {
        if (event.target === loginToggle) {
            try {
                await SettingsService.SetLaunchAtLogin(loginToggle.checked);
                flash(loginToggle.checked ? "Starts at login" : "Won't start at login");
            } catch (err) {
                loginToggle.checked = !loginToggle.checked;
                flash(`Login item failed: ${err}`);
            }
            return;
        }
        await save();
    });

    for (const {key} of kinds) {
        const fs = fieldsets.get(key)!;
        const select = field<HTMLSelectElement>(key, "soundName");
        fs.querySelector(".test")!.addEventListener("click", () => SettingsService.Test(key));
        fs.querySelector(".preview")!.addEventListener("click", () => {
            SettingsService.PreviewSound(select.value).catch((err) => flash(`${err}`));
        });
        fs.querySelector(".add-sound")!.addEventListener("click", async () => {
            try {
                const name = await SettingsService.AddSound();
                if (!name) return;
                await loadSounds();
                select.value = name;
                await save();
            } catch (err) {
                flash(`Add failed: ${err}`);
            }
        });
        fs.querySelector(".remove-sound")!.addEventListener("click", async () => {
            const name = select.value;
            try {
                await SettingsService.RemoveSound(name);
                await loadSounds();
                for (const {key: k} of kinds) {
                    const s = field<HTMLSelectElement>(k, "soundName");
                    if (s.value === name || !s.value) s.value = defaultSounds[k];
                }
                await save();
            } catch (err) {
                flash(`Remove failed: ${err}`);
            }
        });
    }

    const agentsEl = document.getElementById("agents")!;
    const prompt = document.getElementById("hooks-prompt")!;
    const agents = new Map<string, AgentInfo>();

    function describe(a: AgentInfo): string {
        if (a.error) return `Can't read its settings: ${a.error}`;
        if (a.status.installed) return `Hooks installed in ${a.status.settings.replace(/^\/Users\/[^/]+/, "~")}`;
        if (a.status.outdated) return "Hooks point at another copy of Botler; reinstall to fix";
        return `Not connected: Botler won't hear from ${a.name}`;
    }

    function renderAgents() {
        agentsEl.replaceChildren(...[...agents.values()].map((a) => {
            const row = document.createElement("div");
            row.className = "row";
            const label = document.createElement("span");
            const status = Object.assign(document.createElement("small"), {textContent: describe(a)});
            status.classList.toggle("warn", !a.status.installed);
            label.append(a.name, status);
            const button = Object.assign(document.createElement("button"), {
                type: "button",
                textContent: a.status.installed ? "Remove" : a.status.outdated ? "Reinstall" : "Install",
                disabled: !!a.error,
            });
            button.classList.toggle("primary", !a.status.installed);
            button.addEventListener("click", () => (a.status.installed ? uninstall(a) : install([a])));
            row.append(label, button);
            return row;
        }));
        const missing = [...agents.values()].filter((a) => !a.status.installed && !a.error);
        document.getElementById("prompt-tools")!.textContent = missing.map((a) => a.name).join(" and ") || "your AI tools";
        prompt.hidden = missing.length === 0 || config.hooksPrompted;
    }

    async function install(list: AgentInfo[]) {
        for (const a of list) {
            try {
                agents.set(a.id, await SettingsService.InstallHooks(a.id));
                config.hooksPrompted = true;
                flash(`Connected to ${a.name}; new sessions pick it up`);
            } catch (err) {
                flash(`Install failed for ${a.name}: ${err}`);
            }
        }
        renderAgents();
        fit();
    }

    async function uninstall(a: AgentInfo) {
        try {
            agents.set(a.id, await SettingsService.UninstallHooks(a.id));
            flash(`Hooks removed from ${a.name}`);
        } catch (err) {
            flash(`Remove failed for ${a.name}: ${err}`);
        }
        renderAgents();
        fit();
    }

    document.getElementById("prompt-install")!.addEventListener("click", () =>
        install([...agents.values()].filter((a) => !a.status.installed && !a.error)));
    document.getElementById("prompt-later")!.addEventListener("click", async () => {
        prompt.hidden = true;
        config.hooksPrompted = true;
        await SettingsService.DeclineHooks().catch((err) => flash(`${err}`));
        fit();
    });
    document.getElementById("request-agent")!.addEventListener("click", () => {
        SettingsService.RequestAgent().catch((err) => flash(`${err}`));
    });

    async function loadHooks() {
        try {
            for (const a of (await SettingsService.Agents()) ?? []) agents.set(a.id, a);
            renderAgents();
        } catch (err) {
            flash(`Can't read AI tool settings: ${err}`);
        }
    }

    const version = SettingsService.Version().then((v) => {
        document.getElementById("version")!.textContent = `Botler ${v}`;
    });

    function fit() {
        const content = Math.ceil(document.querySelector("main")!.getBoundingClientRect().height);
        SettingsService.FitHeight(content, window.innerHeight);
    }

    // Re-check the hooks whenever the window comes back, since `botler hooks` may have changed them meanwhile.
    window.addEventListener("focus", () => {
        if (config) loadHooks().then(fit);
    });

    Promise.all([load().then(loadHooks).catch((err) => flash(`Load failed: ${err}`)), version]).finally(() => {
        fit();
        requestAnimationFrame(() => document.body.classList.add("ready"));
    });
}
