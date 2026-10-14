import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { paths, useGroupId } from '../groups'

// Ссылки на страницы текущей группы для рендеров колонок таблиц, которые объявлены вне компонентов.

interface LinkProps {
  className?: string
  title?: string
  children: ReactNode
}

export function PlayerLink({ steamId, ...rest }: LinkProps & { steamId: string }) {
  return <Link to={paths.player(useGroupId(), steamId)} {...rest} />
}

export function SessionLink({ id, ...rest }: LinkProps & { id: number }) {
  return <Link to={paths.session(useGroupId(), id)} {...rest} />
}

export function MatchLink({ id, ...rest }: LinkProps & { id: number }) {
  return <Link to={paths.match(useGroupId(), id)} {...rest} />
}
