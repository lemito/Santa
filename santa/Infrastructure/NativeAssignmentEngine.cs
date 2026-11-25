using System.Runtime.InteropServices;
using santa.Application;

namespace santa.Infrastructure;

public sealed class NativeAssignmentEngine : IAssignmentEngine
{
    private const uint AbiVersion = 2;
    [StructLayout(LayoutKind.Sequential)]
    private readonly record struct Pair(uint Giver, uint Receiver);
    [StructLayout(LayoutKind.Sequential)]
    private readonly record struct Budget(ulong Attempts, ulong Work, ulong Memory);
    [DllImport("santa_native", CallingConvention = CallingConvention.Cdecl)]
    private static extern uint santa_abi_version();
    [DllImport("santa_native", CallingConvention = CallingConvention.Cdecl)]
    private static extern int santa_draw(uint n, [In] Pair[] pairs, ulong count,
        Budget budget, [Out] uint[] output, ulong outputCount);

    public EngineResult Draw(EngineInput input)
    {
        if (input.Participants.Length < 2 || input.Participants.Distinct().Count() != input.Participants.Length)
            return new(EngineStatus.InvalidInput);
        var indices = input.Participants.Select((id, index) => (id, index))
            .ToDictionary(x => x.id, x => checked((uint)x.index));
        var pairs = new List<Pair>();
        foreach (var exclusion in input.Exclusions)
        {
            if (!indices.TryGetValue(exclusion.First, out var a) ||
                !indices.TryGetValue(exclusion.Second, out var b) || a == b)
                return new(EngineStatus.InvalidInput);
            pairs.Add(new(a, b)); pairs.Add(new(b, a));
        }
        if (santa_abi_version() != AbiVersion) throw new InvalidOperationException("Unsupported Santa ABI");
        var normalized = pairs.Distinct().ToArray();
        var output = new uint[input.Participants.Length];
        var status = (EngineStatus)santa_draw(checked((uint)output.Length), normalized,
            (ulong)normalized.Length, new(100000, 100000000, 128 * 1024 * 1024), output, (ulong)output.Length);
        if (status != EngineStatus.Success) return new(status);
        // Independent managed validation, including bijection and exclusions.
        if (output.Distinct().Count() != output.Length || output.Where((r, i) => r >= output.Length || r == i).Any() ||
            normalized.Any(p => output[p.Giver] == p.Receiver))
            throw new InvalidOperationException("Native result violated assignment invariants");
        return new(status, indices.ToDictionary(x => x.Key, x => input.Participants[output[x.Value]]));
    }
}
