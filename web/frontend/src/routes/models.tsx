import { createFileRoute } from "@tanstack/react-router"
import { useTranslation } from "react-i18next"

import { PageHeader } from "@/components/page-header"

export const Route = createFileRoute("/models")({
  component: ModelsPage,
})

function ModelsPage() {
  const { t } = useTranslation()
  return (
    <div className="flex h-full flex-col">
      <PageHeader title={t("navigation.models", "Models")} />
      <div className="flex flex-1 items-center justify-center p-8">
        <div className="text-center">
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("navigation.models", "Models")}
          </h1>
          <p className="text-muted-foreground mt-2 text-sm">
            {t("pages.models.description", "Manage AI models here.")}
          </p>
        </div>
      </div>
    </div>
  )
}
