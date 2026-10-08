import { Skeleton } from "@/components/ui/Skeleton";

export default function MonitoramentoLoading() {
  return (
    <div role="status" aria-live="polite" className="flex flex-col gap-6">
      <span className="sr-only">Carregando…</span>
      <Skeleton className="h-28 w-full" />
      <Skeleton className="h-40 w-full" />
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-36 w-full" />
      </div>
    </div>
  );
}
