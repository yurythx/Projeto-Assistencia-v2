"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Building, Mail, Phone } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { Badge } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import { useApiQuery } from "@/lib/api/swr";
import type { Person } from "@/lib/nexus/types";

export default function PessoaPage() {
  const { id } = useParams<{ id: string }>();
  const person = useApiQuery<Person>(`v1/directory/people/${encodeURIComponent(id)}`);
  const p = person.data;

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-6">
      <Link href="/diretorio" className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground">
        <ArrowLeft size={14} aria-hidden="true" /> Diretório
      </Link>
      <DataState loading={person.isLoading} error={person.error} empty={!p}>
        {p && (
          <Card className="flex flex-col gap-3 p-6">
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-semibold">{p.name || p.username}</h1>
              {p.from_ad && <Badge tone="info">AD</Badge>}
            </div>
            {p.job_title && <p className="text-muted">{p.job_title}</p>}
            <ul className="flex flex-col gap-2 text-sm">
              {(p.departamento || p.unidade) && (
                <li className="flex items-center gap-2">
                  <Building size={14} aria-hidden="true" /> {[p.departamento, p.unidade].filter(Boolean).join(" · ")}
                </li>
              )}
              {p.email && (
                <li className="flex items-center gap-2">
                  <Mail size={14} aria-hidden="true" />
                  <a href={`mailto:${p.email}`} className="text-primary hover:underline">
                    {p.email}
                  </a>
                </li>
              )}
              {(p.phone || p.extension) && (
                <li className="flex items-center gap-2">
                  <Phone size={14} aria-hidden="true" /> {p.phone} {p.extension && `· ramal ${p.extension}`}
                </li>
              )}
            </ul>
            {p.bio && <p className="whitespace-pre-line text-sm">{p.bio}</p>}
          </Card>
        )}
      </DataState>
    </div>
  );
}
