import { Component, type ErrorInfo, type ReactNode } from 'react'
import { Button } from '@/shared/ui/Button'
import styles from './ErrorBoundary.module.css'

type State = { failed: boolean }

export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  override state: State = { failed: false }

  static getDerivedStateFromError(): State {
    return { failed: true }
  }

  override componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error(error, info.componentStack)
  }

  override render() {
    if (!this.state.failed) return this.props.children
    return (
      <div className={styles.wrap}>
        <h1>Something broke</h1>
        <p>Reload the page to keep shopping.</p>
        <Button onClick={() => window.location.reload()}>Reload</Button>
      </div>
    )
  }
}
