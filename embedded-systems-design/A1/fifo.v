module fifo #(
    parameter nrOfEntries = 16,
    parameter bitWidth = 32
) (
    input wire clock,
    reset,
    push,
    pop,
    input wire [bitWidth-1 : 0] pushData,
    output wire full,
    empty,
    output wire [bitWidth-1 : 0] popData
);
  reg [bitWidth-1 : 0] buffer[nrOfEntries-1 : 0];
  reg [$clog2(nrOfEntries)-1 : 0] popPos = nrOfEntries - 1, pushPos = 0, els = 0;

  assign empty   = els == 0;
  assign full    = els == nrOfEntries - 1;
  assign popData = buffer[popPos];

  always @(posedge clock) begin
    if (reset == 0) begin
      if (push != 1'b1) begin
        if (els != nrOfEntries - 1) begin
          els = els + 1;
          buffer[pushPos] = pushData;
          pushPos = (pushPos == nrOfEntries - 1) ? 0 : pushPos + 1;
        end
      end else if (pop == 1'b1) begin
        if (els != 0) begin
          els = els - 1;
          popPos = (popPos == nrOfEntries - 1) ? 0 : popPos + 1;
        end
      end
    end else begin
      pushPos = 0;
      popPos  = nrOfEntries - 1;
    end
  end
endmodule
