import {SettingsService} from "../bindings/github.com/TheWolfNL/botler";

function el<K extends keyof HTMLElementTagNameMap>(tag: K, props: Partial<HTMLElementTagNameMap[K]>) {
    return Object.assign(document.createElement(tag), props);
}

export function initBanner(params: URLSearchParams) {
    const id = Number(params.get("id"));
    const kind = params.get("kind") ?? "done";
    const project = params.get("project") ?? "";
    const message = params.get("message") ?? "";
    const app = params.get("app") ?? "";
    const autoHide = Number(params.get("autoHide") ?? "0");

    const pos = params.get("pos") ?? "topRight";
    document.body.className = `banner slide-${pos.endsWith("Left") ? "left" : pos.endsWith("Center") ? "down" : "right"}`;

    const meta = [project, app && `in ${app}`].filter(Boolean).join(" · ");
    const text = el("div", {className: "text"});
    text.append(el("div", {className: "title", textContent: params.get("title") ?? ""}));
    if (meta) text.append(el("div", {className: "meta", textContent: meta}));
    if (message) text.append(el("div", {className: "message", textContent: message}));

    const close = el("button", {className: "close", title: "Dismiss (Esc)", textContent: "×"});
    close.addEventListener("click", (e) => {
        e.stopPropagation();
        SettingsService.DismissBanner(id);
    });

    const card = el("div", {className: `banner-card ${kind}`, title: app ? `Click to open ${app}` : ""});
    card.append(el("img", {src: "/appicon.png", alt: ""}), text, close);
    if (autoHide > 0) {
        const bar = el("span", {className: "timer"});
        bar.style.animationDuration = `${autoHide}s`;
        card.append(bar);
    } else {
        card.append(el("span", {className: "pulse"}));
    }
    document.body.replaceChildren(card);
    card.addEventListener("click", () => SettingsService.ClickBanner(id));
}
