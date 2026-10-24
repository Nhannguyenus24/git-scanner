import React, { createContext, useCallback, useContext, useState } from 'react'
import Alert from '@mui/material/Alert'
import Snackbar from '@mui/material/Snackbar'

type Notify = (msg: string, severity?: 'success' | 'error' | 'info') => void
const ToastCtx = createContext<Notify>(() => {})

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toast, setToast] = useState<{ msg: string; severity: 'success' | 'error' | 'info'; key: number } | null>(null)
  const notify = useCallback<Notify>((msg, severity = 'success') => setToast({ msg, severity, key: Date.now() }), [])
  return (
    <ToastCtx.Provider value={notify}>
      {children}
      <Snackbar key={toast?.key} open={!!toast} autoHideDuration={3500} onClose={() => setToast(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        {toast ? <Alert severity={toast.severity} variant="filled" onClose={() => setToast(null)}>{toast.msg}</Alert> : undefined}
      </Snackbar>
    </ToastCtx.Provider>
  )
}

export const useToast = () => useContext(ToastCtx)
