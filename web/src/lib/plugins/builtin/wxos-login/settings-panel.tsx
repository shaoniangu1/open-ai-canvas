import { useEffect, useState } from "react";
import { App, Button, Form, Input } from "antd";
import { getWxosSetting, saveWxosSetting, type WxosSetting } from "@/services/api/wxos";
import { SettingsSectionCard } from "@/pages/admin/components/admin-ui";

export function WxosSettingsPanel() {
    const { message } = App.useApp();
    const [form] = Form.useForm<{ issuer: string; secret: string }>();
    const [setting, setSetting] = useState<WxosSetting | null>(null);
    const [error, setError] = useState("");
    const [saving, setSaving] = useState(false);
    const [loading, setLoading] = useState(true);
    async function load() {
        setLoading(true);
        setError("");
        try {
            const data = await getWxosSetting();
            setSetting(data.setting);
            form.setFieldsValue({ issuer: data.setting.issuer, secret: "" });
        } catch (cause) {
            setError(cause instanceof Error ? cause.message : "读取失败");
        } finally {
            setLoading(false);
        }
    }
    useEffect(() => {
        void load();
    }, []);
    async function save(input: { issuer: string; secret: string }) {
        setSaving(true);
        setError("");
        try {
            const data = await saveWxosSetting(input);
            setSetting(data.setting);
            form.setFieldsValue({ ...input, issuer: data.setting.issuer, secret: "" });
            message.success("wxOS 接入配置已保存");
        } catch (cause) {
            setError(cause instanceof Error ? cause.message : "保存失败");
        } finally {
            setSaving(false);
        }
    }
    return (
        <SettingsSectionCard title="wxOS 登录接入" description="允许 wxOS 已登录用户无感进入画布。影策独立登录与注册保持可用。">
            {error ? <p role="alert">{error}</p> : null}
            <Button onClick={() => void load()} loading={loading} disabled={saving}>
                重新读取
            </Button>
            {setting ? (
                <Form form={form} layout="vertical" disabled={saving || loading} onFinish={(input) => void save(input)}>
                    <Form.Item name="issuer" label="wxOS 身份来源编号">
                        <Input readOnly={Boolean(setting.issuer)} placeholder="从 wxOS 后台复制，保存后不可修改" />
                    </Form.Item>
                    <Form.Item name="secret" label="接入密钥">
                        <Input.Password autoComplete="new-password" placeholder={setting.hasSecret ? "已配置，留空保留" : "与 wxOS 后台填写相同密钥"} />
                    </Form.Item>
                    <p>在插件中心启用后，允许 wxOS 自动创建普通用户及空钱包，不按邮箱或名称合并账号。插件停用会阻止新的 wxOS 登录；已经建立的影策会话按原会话规则退出。</p>
                    <Button type="primary" htmlType="submit" loading={saving}>
                        保存
                    </Button>
                </Form>
            ) : null}
        </SettingsSectionCard>
    );
}
