import { http } from "./request";

export interface WxosSetting {
    issuer: string;
    hasSecret: boolean;
}
export const getWxosSetting = () => http.get<{ setting: WxosSetting }>("/admin/plugins/wxos-login/settings");
export const saveWxosSetting = (input: { issuer: string; secret: string }) => http.patch<{ setting: WxosSetting }>("/admin/plugins/wxos-login/settings", input);
