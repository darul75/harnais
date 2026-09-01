import { useEffect, useState } from "react";

import {
  getSettings,
  testSettings,
  updateSettings,
} from "./api";

import type {
  ProviderSettings,
  Settings,
} from "./types";

export function SettingsView() {
  const [settings, setSettings] =
    useState<Settings | null>(null);

  const [error, setError] =
    useState<string | null>(null);

  const [success, setSuccess] =
    useState<string | null>(null);

  const [busy, setBusy] =
    useState<string | null>(null);

  const [revealed, setRevealed] =
    useState<Record<string, boolean>>({});

  // ------------------------------------------------------------
  // Load settings
  // ------------------------------------------------------------

  useEffect(() => {
    let disposed = false;

    getSettings()
      .then((data) => {
        if (disposed) {
          return;
        }
        setSettings(data);
      })
      .catch((err) => {
        if (disposed) {
          return;
        }
        setError(String(err));
      });

    return () => {
      disposed = true;
    };
  }, []);

  // ------------------------------------------------------------
  // Helpers
  // ------------------------------------------------------------

  function setValue(
    providerId: string,
    key: string,
    value: string,
  ) {
    setSettings((prev) => {
      if (!prev) {
        return prev;
      }

      return {
        ...prev,
        providers: prev.providers.map(
          (provider) =>
            provider.id ===
            providerId
              ? {
                  ...provider,
                  values: {
                    ...provider.values,
                    [key]: value,
                  },
                }
              : provider,
        ),
      };
    });
  }

  function toggleReveal(
    providerId: string,
    key: string,
  ) {
    setRevealed((prev) => ({
      ...prev,
      [`${providerId}:${key}`]:
        !prev[`${providerId}:${key}`],
    }));
  }

  // ------------------------------------------------------------
  // Actions
  // ------------------------------------------------------------

  async function save(
    provider: ProviderSettings,
  ) {
    setBusy(`save:${provider.id}`);
    setError(null);
    setSuccess(null);

    try {
      const updated = await updateSettings({
        [provider.id]: provider.values,
      });

      setSettings(updated);
      setSuccess(
        `${provider.label} settings saved.`,
      );
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  }

  async function test(
    provider: ProviderSettings,
  ) {
    setBusy(`test:${provider.id}`);
    setError(null);
    setSuccess(null);

    try {
      await testSettings(
        provider.id,
        provider.values,
      );

      setSuccess(
        `${provider.label} connection OK.`,
      );
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  }

  // ------------------------------------------------------------
  // Render
  // ------------------------------------------------------------

  const configuredCount =
    settings?.providers.filter(
      (provider) => provider.configured,
    ).length ?? 0;

  return (
    <div className="settings-view">
      <section className="panel">
        <div className="panel-header">
          <h2>
            Settings
          </h2>

          <span>
            Providers & API keys
          </span>
        </div>

        {error && (
          <div className="error">
            {error}
          </div>
        )}

        {success && (
          <div className="success">
            {success}
          </div>
        )}

        {settings &&
          configuredCount === 0 && (
            <div className="settings-hint">
              No providers configured yet.
              Add your API keys below to
              start running workflows.
            </div>
          )}

        <div className="settings-providers">
          {settings?.providers.map(
            (provider) => (
              <div
                key={provider.id}
                className="settings-provider"
              >
                <div className="settings-provider-header">
                  <h3>
                    {provider.label}
                  </h3>

                  <span
                    className={`settings-badge ${
                      provider.configured
                        ? "settings-badge-ok"
                        : ""
                    }`}
                  >
                    {provider.configured
                      ? "Configured"
                      : "Not configured"}
                  </span>
                </div>

                <div className="settings-fields">
                  {provider.fields.map(
                    (field) => {
                      const revealKey =
                        `${provider.id}:${field.key}`;

                      const isSecret =
                        field.type ===
                        "secret";

                      return (
                        <label
                          key={field.key}
                          className="settings-field"
                        >
                          <span className="settings-field-label">
                            {field.label}
                          </span>

                          <div className="settings-field-input">
                            <input
                              type={
                                isSecret &&
                                !revealed[
                                  revealKey
                                ]
                                  ? "password"
                                  : "text"
                              }
                              value={
                                provider
                                  .values[
                                  field.key
                                ] ?? ""
                              }
                              onChange={(
                                event,
                              ) =>
                                setValue(
                                  provider.id,
                                  field.key,
                                  event.target
                                    .value,
                                )
                              }
                              placeholder={
                                field.placeholder
                              }
                              autoComplete="off"
                              spellCheck={false}
                            />

                            {isSecret && (
                              <button
                                type="button"
                                onClick={() =>
                                  toggleReveal(
                                    provider.id,
                                    field.key,
                                  )
                                }
                                aria-label={
                                  revealed[
                                    revealKey
                                  ]
                                    ? "Hide key"
                                    : "Show key"
                                }
                              >
                                {revealed[
                                  revealKey
                                ]
                                  ? "Hide"
                                  : "Show"}
                              </button>
                            )}
                          </div>
                        </label>
                      );
                    },
                  )}
                </div>

                <div className="settings-actions">
                  <button
                    type="button"
                    onClick={() =>
                      test(provider)
                    }
                    disabled={busy !== null}
                  >
                    {busy ===
                    `test:${provider.id}`
                      ? "Testing..."
                      : "Test"}
                  </button>

                  <button
                    type="button"
                    className="settings-save"
                    onClick={() =>
                      save(provider)
                    }
                    disabled={busy !== null}
                  >
                    {busy ===
                    `save:${provider.id}`
                      ? "Saving..."
                      : "Save"}
                  </button>
                </div>
              </div>
            ),
          )}

          {!settings && (
            <div className="empty">
              Loading settings...
            </div>
          )}
        </div>
      </section>
    </div>
  );
}