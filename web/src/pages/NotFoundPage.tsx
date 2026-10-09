import { ButtonLink } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Page } from '../components/ui/Layout'

export function NotFoundPage() {
  return (
    <Page className="md:pt-24">
      <Card as="div" className="mx-auto w-full max-w-[560px]">
        <div className="flex flex-col items-center gap-2 px-8 py-14 text-center">
          <span className="font-mono text-[56px] leading-[60px] font-semibold text-fg-faint">404</span>
          <h1 className="m-0 mt-2 text-[24px] leading-[30px] font-bold tracking-[-0.01em] md:text-page">Страница не найдена</h1>
          <p className="m-0 text-fg-muted">Возможно, сессию или матч удалили, или в ссылке опечатка.</p>
          <div className="mt-3 flex flex-wrap justify-center gap-2">
            <ButtonLink to="/" variant="primary">
              К списку сессий
            </ButtonLink>
            <ButtonLink to="/players" variant="secondary">
              Все игроки
            </ButtonLink>
          </div>
        </div>
      </Card>
    </Page>
  )
}
