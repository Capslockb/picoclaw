import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"

export const Route = createFileRoute("/providers")({
  component: ProvidersPage,
})

function ProvidersPage() {
  const { t } = useTranslation()
  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("navigation.providers", "Providers")} />
      <div className="flex flex-1 items-center justify-center p-8">
        <div className="text-center">
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("navigation.providers", "Providers")}
          </h1>
          <p className="text-muted-foreground mt-2 text-sm">
            {t(
              "pages.providers.description",
              "Manage AI model providers and configurations.",
            )}
          </p>
        </div>
      </div>
    </div>
  )
}
