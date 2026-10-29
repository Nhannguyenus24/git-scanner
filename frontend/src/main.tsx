import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { ColorModeProvider } from './store/mode'
import { I18nProvider } from './i18n'
import { PreferencesProvider } from './store/preferences'
import { SettingsProvider } from './store/settings'
import { ToastProvider } from './store/toast'

// PreferencesProvider wraps ColorModeProvider: the theme is built from the
// stored preset, density and motion preference, so it has to be able to read
// them before it builds anything.
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <PreferencesProvider>
      <ColorModeProvider>
        <I18nProvider>
          <SettingsProvider>
            <ToastProvider>
              <App />
            </ToastProvider>
          </SettingsProvider>
        </I18nProvider>
      </ColorModeProvider>
    </PreferencesProvider>
  </React.StrictMode>,
)
