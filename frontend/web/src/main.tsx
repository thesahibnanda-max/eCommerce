import '@fontsource-variable/inter/wght.css'
import '@fontsource-variable/fredoka/wght.css'
import './shared/styles/index.css'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './app/App'
import { AppProviders } from './app/AppProviders'

const root = document.getElementById('root')
if (root) {
  createRoot(root).render(
    <StrictMode>
      <AppProviders>
        <App />
      </AppProviders>
    </StrictMode>,
  )
}
