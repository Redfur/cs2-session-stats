import { CircleAlert, LoaderCircle } from 'lucide-react'
import { Button, ButtonLink } from './ui/Button'
import { Card } from './ui/Card'
import { EmptyState } from './ui/EmptyState'
import { Page } from './ui/Layout'

// PageError — страница не загрузилась: текст ответа сервера, повтор и выход к списку.
export function PageError({
  title,
  error,
  onRetry,
  back,
}: {
  title: string
  error: string
  onRetry: () => void
  back: { to: string; label: string }
}) {
  return (
    <Page>
      <Card as="div">
        <EmptyState
          as="h1"
          tone="error"
          className="py-14"
          icon={<CircleAlert />}
          title={title}
          text={<>Сервер ответил: «{error}». Попробуйте ещё раз или вернитесь к списку.</>}
          actions={
            <>
              <ButtonLink to={back.to} variant="secondary">
                {back.label}
              </ButtonLink>
              <Button variant="primary" onClick={onRetry}>
                Повторить
              </Button>
            </>
          }
        />
      </Card>
    </Page>
  )
}

export function PageLoading() {
  return (
    <Page>
      <p className="m-0 inline-flex items-center gap-2 text-fg-muted" role="status">
        <LoaderCircle className="size-4 animate-spin" aria-hidden />
        Загрузка…
      </p>
    </Page>
  )
}
