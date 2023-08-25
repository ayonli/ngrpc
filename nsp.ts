
import { applyMagic } from "js-magic";
import type App from "./app";

@applyMagic
class ChainingProxy {
    protected __target: string;
    protected __app: App;
    protected __children: { [prop: string]: ChainingProxy; } = {};

    constructor(target: string, app: App) {
        this.__target = target;
        this.__app = app;
    }

    protected __get(prop: string | symbol) {
        if (prop in this) {
            return this[prop];
        } else if (prop in this.__children) {
            return this.__children[String(prop)];
        } else if (typeof prop !== "symbol") {
            return (this.__children[prop] = createChainingProxy(
                (this.__target ? this.__target + "." : "") + String(prop),
                this.__app
            ));
        }
    }

    protected __has(prop: string | symbol) {
        return (prop in this) || (prop in this.__children);
    }
}

export function createChainingProxy(target: string, app: App) {
    const chain: ChainingProxy = function (data: any = null) {
        const index = target.lastIndexOf(".");
        const serviceName = target.slice(0, index);
        const method = target.slice(index + 1);
        const ins = app.getServiceClient(serviceName, data);

        if (typeof ins[method] === "function") {
            return ins[method](data);
        } else {
            throw new TypeError(`${target} is not a function`);
        }
    } as any;

    Object.setPrototypeOf(chain, ChainingProxy.prototype);
    Object.assign(chain, { __target: target, __app: app, __children: {} });

    return applyMagic(chain as any, true);
}
