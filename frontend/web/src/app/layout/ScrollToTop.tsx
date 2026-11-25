import { useLayoutEffect } from 'react'
import { useLocation, useNavigationType } from 'react-router'

export function ScrollToTop() {
  const { pathname } = useLocation()
  const navigation = useNavigationType()
  useLayoutEffect(() => {
    if (navigation !== 'POP') window.scrollTo(0, 0)
  }, [pathname, navigation])
  return null
}
