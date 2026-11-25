import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { CalendarHeart, Mail, MapPin, Phone, UserRound } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { Link, NavLink, useParams } from 'react-router'
import { errorMessage, isApiError } from '@/shared/api/errors'
import { getInfoPage, infoLinks, type InfoContact, type InfoSection } from '@/shared/api/info'
import { Button } from '@/shared/ui/Button'
import { Flower, Heart, Mascot, Sparkle, Sticker, toneAt } from '@/shared/ui/doodles'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Skeleton } from '@/shared/ui/Skeleton'
import styles from './InfoPage.module.css'

function sectionId(index: number): string {
  return `section-${index + 1}`
}

export default function InfoPage() {
  const { slug = '' } = useParams()
  const query = useQuery({
    queryKey: ['info', slug],
    queryFn: ({ signal }) => getInfoPage(slug, signal),
    enabled: slug.length > 0,
    staleTime: 30 * 60_000,
  })
  const navRef = useRef<HTMLElement>(null)
  useEffect(() => {
    navRef.current
      ?.querySelector('[aria-current="page"]')
      ?.scrollIntoView({ block: 'nearest', inline: 'center' })
  }, [slug])

  return (
    <div className={styles.page}>
      <aside className={styles.sidebar}>
        <nav ref={navRef} className={styles.policyNav} aria-label="Policies">
          <span className={styles.navTitle}>Policies ✿</span>
          {infoLinks.map((link, index) => (
            <NavLink
              key={link.slug}
              to={`/info/${link.slug}`}
              className={({ isActive }) =>
                clsx(styles.navLink, styles[toneAt(index)], isActive && styles.navActive)
              }
            >
              {link.label}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className={styles.content}>
        {query.isPending ? (
          <InfoSkeleton />
        ) : query.isError ? (
          <EmptyState
            mood={isApiError(query.error, 'page_not_found') ? 'surprised' : 'sleepy'}
            tone="sky"
            title={
              isApiError(query.error, 'page_not_found')
                ? 'We could not find that page'
                : 'Could not load this page'
            }
            message={
              isApiError(query.error, 'page_not_found')
                ? 'Pick one of our policies instead.'
                : errorMessage(query.error)
            }
            action={
              <Button variant="ghost" onClick={() => void query.refetch()}>
                Try again
              </Button>
            }
          />
        ) : (
          <article className={styles.article}>
            <header className={styles.hero}>
              <span className={styles.eyebrow}>
                <CalendarHeart size={15} /> Effective {query.data.page.effective_date}
              </span>
              <h1 className={styles.title}>{query.data.page.title}</h1>
              <p className={styles.summary}>{query.data.page.summary}</p>
              <Sticker className={styles.s1} tilt={-10}>
                <Heart size={30} />
              </Sticker>
              <Sticker className={styles.s2} tilt={12} delay={0.8}>
                <Sparkle size={26} />
              </Sticker>
              <Sticker className={styles.s3} tilt={6} delay={1.4}>
                <Flower size={28} tone="mint" />
              </Sticker>
            </header>

            {query.data.page.sections.length > 3 && (
              <nav className={styles.toc} aria-label="On this page">
                {query.data.page.sections.map((section, index) => (
                  <a key={section.heading} href={`#${sectionId(index)}`} className={styles.tocLink}>
                    {section.heading}
                  </a>
                ))}
              </nav>
            )}

            {query.data.page.sections.map((section, index) => (
              <Section key={section.heading} section={section} index={index} />
            ))}

            <ContactCard contact={query.data.contact} />
          </article>
        )}
      </div>
    </div>
  )
}

function Section({ section, index }: { section: InfoSection; index: number }) {
  return (
    <section id={sectionId(index)} className={styles.section}>
      <h2 className={styles.sectionTitle}>
        <span className={clsx(styles.number, styles[toneAt(index)])}>{index + 1}</span>
        {section.heading}
      </h2>
      {section.paragraphs?.map((text) => (
        <p key={text} className={styles.paragraph}>
          {text}
        </p>
      ))}
      {section.bullets && section.bullets.length > 0 && (
        <ul className={styles.bullets}>
          {section.bullets.map((text) => (
            <li key={text}>{text}</li>
          ))}
        </ul>
      )}
    </section>
  )
}

function ContactCard({ contact }: { contact: InfoContact }) {
  return (
    <section className={styles.contact}>
      <Mascot size={78} mood="happy" wave className={styles.contactMascot} />
      <div className={styles.contactBody}>
        <h2 className={styles.contactTitle}>Still have questions?</h2>
        <p className={styles.contactText}>We are real people and we love hearing from you.</p>
        <ul className={styles.contactList}>
          <li>
            <Mail size={16} />
            <a href={`mailto:${contact.email}`}>{contact.email}</a>
          </li>
          <li>
            <Phone size={16} />
            <a href={`tel:${contact.phone.replace(/\s+/g, '')}`}>{contact.phone}</a>
          </li>
          <li>
            <UserRound size={16} />
            <span>Grievance Officer: {contact.grievance_officer}</span>
          </li>
          <li>
            <MapPin size={16} />
            <span>{contact.address}</span>
          </li>
        </ul>
        <Link to="/" className={styles.back}>
          ← Back to shopping
        </Link>
      </div>
    </section>
  )
}

function InfoSkeleton() {
  return (
    <div className={styles.article} aria-busy>
      <Skeleton style={{ height: 170, borderRadius: 'var(--radius-xl)' }} />
      {Array.from({ length: 4 }, (_, i) => (
        <Skeleton key={i} style={{ height: 110, borderRadius: 'var(--radius-lg)' }} />
      ))}
    </div>
  )
}
