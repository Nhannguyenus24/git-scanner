import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { ColorModeProvider } from './store/mode'
import { I18nProvider } from './i18n'
import { SettingsProvider } from './store/settings'
import { ToastProvider } from './store/toast'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ColorModeProvider>
      <I18nProvider>
        <SettingsProvider>
          <ToastProvider>
            <App />
          </ToastProvider>
        </SettingsProvider>
      </I18nProvider>
    </ColorModeProvider>
  </React.StrictMode>,
)
