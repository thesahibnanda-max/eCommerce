import { Suspense } from 'react'
import { Outlet } from 'react-router'
import { Footer } from './Footer'
import { PageFallback } from './PageFallback'
import { TabBar } from './TabBar'
import { TopBar } from './TopBar'
import styles from './Shell.module.css'

export function Shell() {
  return (
    <div className={styles.shell}>
      <TopBar />
      <main className={styles.main}>
        <Suspense fallback={<PageFallback />}>
          <Outlet />
        </Suspense>
      </main>
      <Footer />
      <TabBar />
    </div>
  )
}
